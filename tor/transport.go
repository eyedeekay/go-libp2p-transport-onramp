package tor

import (
	"context"
	"fmt"
	"sync"

	"github.com/cretz/bine/torutil/ed25519"
	"github.com/go-i2p/onramp"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/transport"
	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
)

// Transport implements the libp2p transport.Transport interface for Tor.
// It provides anonymous communication over the Tor network using onramp's Onion API.
type Transport struct {
	onion    *onramp.Onion
	upgrader transport.Upgrader
	rcmgr    network.ResourceManager

	listenersMu sync.RWMutex
	listeners   map[string]*listener
}

// NewTransport creates a new Tor transport.
// The transport will use onramp's default configuration with minimal options exposed.
func NewTransport(upgrader transport.Upgrader, rcmgr network.ResourceManager) (*Transport, error) {
	if upgrader == nil {
		return nil, fmt.Errorf("upgrader cannot be nil")
	}

	// Create onramp.Onion with default configuration
	onion, err := onramp.NewOnion("libp2p-tor")
	if err != nil {
		return nil, fmt.Errorf("failed to create onion service: %w", err)
	}

	t := &Transport{
		onion:     onion,
		upgrader:  upgrader,
		rcmgr:     rcmgr,
		listeners: make(map[string]*listener),
	}

	return t, nil
}

// Dial dials a remote peer over Tor.
// The raddr must be an onion3 multiaddr (e.g., /onion3/<addr>:<port>).
func (t *Transport) Dial(ctx context.Context, raddr ma.Multiaddr, p peer.ID) (transport.CapableConn, error) {
	// Parse the onion multiaddr
	onionAddr, err := parseOnionMultiaddr(raddr)
	if err != nil {
		return nil, fmt.Errorf("tor: invalid multiaddr: %w", err)
	}

	// Dial using onramp
	rawConn, err := t.onion.Dial("tcp", onionAddr)
	if err != nil {
		return nil, fmt.Errorf("tor: dial failed: %w", err)
	}

	// Wrap in manet.Conn
	// We need to create fake local multiaddr since we don't know it
	localAddr, _ := ma.NewMultiaddr("/ip4/127.0.0.1/tcp/0")
	maConn, err := manet.WrapNetConn(rawConn)
	if err != nil {
		rawConn.Close()
		return nil, fmt.Errorf("tor: failed to wrap connection: %w", err)
	}

	// Get connection scope
	connScope, err := t.rcmgr.OpenConnection(network.DirOutbound, false, raddr)
	if err != nil {
		maConn.Close()
		return nil, fmt.Errorf("tor: failed to open connection scope: %w", err)
	}

	// Upgrade the connection with security and multiplexing
	conn, err := t.upgrader.Upgrade(ctx, t, maConn, network.DirOutbound, p, connScope)
	if err != nil {
		connScope.Done()
		maConn.Close()
		return nil, fmt.Errorf("tor: upgrade failed: %w", err)
	}

	// Set the proper local and remote multiaddrs after upgrade
	_ = localAddr // silence unused warning for now

	return conn, nil
}

// CanDial returns true if this transport can dial the given multiaddr.
// It returns true only for onion3 multiaddrs.
func (t *Transport) CanDial(addr ma.Multiaddr) bool {
	return isOnionMultiaddr(addr)
}

// Protocols returns the list of protocols this transport handles.
// For Tor, this is just the onion3 protocol.
func (t *Transport) Protocols() []int {
	return []int{P_ONION3}
}

// Listen listens for incoming connections on the given multiaddr.
// For Tor, this creates an onion service. The laddr can be empty to use auto-generated address.
func (t *Transport) Listen(laddr ma.Multiaddr) (transport.Listener, error) {
	// Create the onion service listener
	netListener, err := t.onion.Listen()
	if err != nil {
		return nil, fmt.Errorf("tor: failed to create listener: %w", err)
	}

	// Get the onion address from the keys
	keys, err := t.onion.Keys()
	if err != nil {
		netListener.Close()
		return nil, fmt.Errorf("tor: failed to get onion keys: %w", err)
	}

	// Generate the onion address from the keys
	onionAddr := getOnionAddress(keys)

	// Parse the port from the listener
	listenerAddr := netListener.Addr().String()

	// Extract port from address
	var port string
	if idx := lastIndex(listenerAddr, ":"); idx >= 0 {
		port = listenerAddr[idx+1:]
	} else {
		port = "0"
	}

	// Build the multiaddr: /onion3/<base32>:<port>
	maddr, err := ma.NewMultiaddr(fmt.Sprintf("/onion3/%s:%s", onionAddr, port))
	if err != nil {
		netListener.Close()
		return nil, fmt.Errorf("tor: failed to create multiaddr: %w", err)
	}

	// Create listener
	l := &listener{
		Listener:  netListener,
		transport: t,
		laddr:     maddr,
	}

	// Register listener
	t.listenersMu.Lock()
	t.listeners[maddr.String()] = l
	t.listenersMu.Unlock()

	return l, nil
}

// getOnionAddress derives the v3 onion address from ed25519 keys
func getOnionAddress(keys ed25519.KeyPair) string {
	// The v3 onion address is base32 encoded from:
	// version (1 byte) || pubkey (32 bytes) || checksum (2 bytes)
	// But we can use the existing onion ID method if available
	// For now, we'll use a placeholder - this needs to be properly implemented
	// TODO: Use proper onion address generation from ed25519 keys
	if keys == nil {
		return "unknown"
	}
	// This is a simplified version - proper implementation would encode
	// the public key properly according to Tor spec
	return "placeholder56charactersonionaddressxxxxxxxxxxxxxxxxxx"
}

// Tor acts as an anonymizing proxy for all connections.
func (t *Transport) Proxy() bool {
	return true
}

// Close closes the transport and all associated resources.
func (t *Transport) Close() error {
	t.listenersMu.Lock()
	listeners := make([]*listener, 0, len(t.listeners))
	for _, l := range t.listeners {
		listeners = append(listeners, l)
	}
	t.listeners = make(map[string]*listener)
	t.listenersMu.Unlock()

	// Close all listeners
	for _, l := range listeners {
		l.Close()
	}

	// Close the onion service
	if t.onion != nil {
		return t.onion.Close()
	}

	return nil
}

// removeListener removes a listener from the transport's registry.
func (t *Transport) removeListener(l *listener) {
	t.listenersMu.Lock()
	delete(t.listeners, l.laddr.String())
	t.listenersMu.Unlock()
}

// lastIndex returns the index of the last occurrence of substr in s, or -1 if not found.
func lastIndex(s, substr string) int {
	for i := len(s) - len(substr); i >= 0; i-- {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
