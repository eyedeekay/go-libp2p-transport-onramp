// Package i2p provides a libp2p transport implementation for I2P using the onramp library.
// It wraps onramp's Garlic API to provide anonymous communication over the I2P network.
package i2p

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/go-i2p/onramp"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/transport"
	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
)

// Transport implements the libp2p transport.Transport interface for I2P.
// It provides anonymous communication over the I2P network using onramp's Garlic API.
type Transport struct {
	garlic   *onramp.Garlic
	upgrader transport.Upgrader
	rcmgr    network.ResourceManager

	listenersMu sync.RWMutex
	listeners   map[string]*listener

	connMu sync.RWMutex
	conns  map[transport.CapableConn]struct{}
}

// TransportConfig holds configuration options for the I2P transport.
type TransportConfig struct {
	// ServiceName is the identifier for the I2P destination.
	// If empty, defaults to "libp2p-i2p".
	ServiceName string

	// SAMAddr is the address of the SAM bridge.
	// If empty, defaults to "127.0.0.1:7656".
	SAMAddr string

	// Options configures I2P tunnel parameters.
	// Common values: onramp.OPT_DEFAULTS (balanced), onramp.OPT_HUGE (more tunnels).
	// If nil, defaults to onramp.OPT_DEFAULTS.
	Options []string
}

// DefaultTransportConfig returns the default configuration for I2P transport.
func DefaultTransportConfig() *TransportConfig {
	return &TransportConfig{
		ServiceName: "libp2p-i2p",
		SAMAddr:     onramp.SAM_ADDR,
		Options:     onramp.OPT_DEFAULTS,
	}
}

// NewTransport creates a new I2P transport with default configuration.
// For custom configuration, use NewTransportWithOptions.
func NewTransport(upgrader transport.Upgrader, rcmgr network.ResourceManager) (*Transport, error) {
	return NewTransportWithOptions(upgrader, rcmgr, nil)
}

// NewTransportWithOptions creates a new I2P transport with custom configuration.
// If config is nil, default configuration is used.
func NewTransportWithOptions(upgrader transport.Upgrader, rcmgr network.ResourceManager, config *TransportConfig) (*Transport, error) {
	if upgrader == nil {
		return nil, fmt.Errorf("upgrader cannot be nil")
	}

	// Use default config if none provided
	if config == nil {
		config = DefaultTransportConfig()
	}

	// Set defaults for empty fields
	serviceName := config.ServiceName
	if serviceName == "" {
		serviceName = "libp2p-i2p"
	}
	samAddr := config.SAMAddr
	if samAddr == "" {
		samAddr = onramp.SAM_ADDR
	}
	options := config.Options
	if options == nil {
		options = onramp.OPT_DEFAULTS
	}

	// Create onramp.Garlic with configuration
	garlic, err := onramp.NewGarlic(serviceName, samAddr, options)
	if err != nil {
		return nil, fmt.Errorf("i2p: failed to create garlic service: %w", err)
	}

	t := &Transport{
		garlic:    garlic,
		upgrader:  upgrader,
		rcmgr:     rcmgr,
		listeners: make(map[string]*listener),
		conns:     make(map[transport.CapableConn]struct{}),
	}

	return t, nil
}

// trackedConn wraps a connection to enable lifecycle tracking.
// When the connection is closed, it automatically removes itself from the transport's registry.
type trackedConn struct {
	transport.CapableConn
	onClose func()
	once    sync.Once // Ensures onClose is called exactly once, even if Close is called multiple times
}

// Close closes the underlying connection and calls the cleanup callback.
// The sync.Once ensures the cleanup callback (onClose) is executed exactly once,
// preventing double-cleanup issues if Close is called multiple times concurrently.
func (c *trackedConn) Close() error {
	err := c.CapableConn.Close()
	c.once.Do(c.onClose)
	return err
}

// trackConnection registers a connection in the transport's registry and returns a wrapper
// that will automatically unregister the connection when closed.
func (t *Transport) trackConnection(conn transport.CapableConn) transport.CapableConn {
	t.connMu.Lock()
	t.conns[conn] = struct{}{}
	t.connMu.Unlock()

	return &trackedConn{
		CapableConn: conn,
		onClose: func() {
			t.removeConnection(conn)
		},
	}
}

// removeConnection removes a connection from the transport's registry.
func (t *Transport) removeConnection(conn transport.CapableConn) {
	t.connMu.Lock()
	delete(t.conns, conn)
	t.connMu.Unlock()
}

// Dial dials a remote peer over I2P.
// The raddr must be a garlic32 multiaddr (e.g., /garlic32/<addr>.b32.i2p or with port).
func (t *Transport) Dial(ctx context.Context, raddr ma.Multiaddr, p peer.ID) (transport.CapableConn, error) {
	// Parse the garlic multiaddr
	dest, port, err := parseGarlicMultiaddr(raddr)
	if err != nil {
		return nil, fmt.Errorf("i2p: invalid multiaddr: %w", err)
	}

	// Dial using onramp
	var rawConn net.Conn
	if port > 0 {
		// Use virtual port dialing - append port to destination
		addr := fmt.Sprintf("%s:%d", dest, port)
		rawConn, err = t.garlic.DialContext(ctx, "tcp", addr)
		if err != nil {
			return nil, fmt.Errorf("i2p: dial with port failed: %w", err)
		}
	} else {
		// Standard dial without virtual ports
		rawConn, err = t.garlic.DialContext(ctx, "tcp", dest)
		if err != nil {
			return nil, fmt.Errorf("i2p: dial failed: %w", err)
		}
	}

	// Wrap in manet.Conn
	maConn, err := manet.WrapNetConn(rawConn)
	if err != nil {
		rawConn.Close()
		return nil, fmt.Errorf("i2p: failed to wrap connection: %w", err)
	}

	// Get connection scope
	connScope, err := t.rcmgr.OpenConnection(network.DirOutbound, false, raddr)
	if err != nil {
		maConn.Close()
		return nil, fmt.Errorf("i2p: failed to open connection scope: %w", err)
	}

	// Upgrade the connection with security and multiplexing
	conn, err := t.upgrader.Upgrade(ctx, t, maConn, network.DirOutbound, p, connScope)
	if err != nil {
		connScope.Done()
		maConn.Close()
		return nil, fmt.Errorf("i2p: upgrade failed: %w", err)
	}

	// Track the connection for proper cleanup on transport Close
	wrappedConn := t.trackConnection(conn)

	return wrappedConn, nil
}

// CanDial returns true if this transport can dial the given multiaddr.
// It returns true only for garlic32 multiaddrs.
// Returns false for all other address types including IP addresses (ip4, ip6),
// DNS addresses, or other protocol multiaddrs that are not garlic32.
func (t *Transport) CanDial(addr ma.Multiaddr) bool {
	return isGarlicMultiaddr(addr)
}

// Listen listens for incoming connections on the given multiaddr.
// For I2P, this creates an I2P destination.
//
// laddr Parameter Behavior:
//
// The laddr parameter is IGNORED by this implementation. The I2P destination address
// is always auto-generated by onramp based on persistent cryptographic keys.
//
// This differs from typical transport.Transport behavior where laddr specifies the
// listening address:
//
//   - In typical transports (TCP, QUIC), laddr determines the bind address (e.g., 0.0.0.0:4001)
//   - In I2P, the destination address is derived from the destination's public key
//
// Why laddr Cannot Be Used:
//
//  1. I2P destinations are cryptographic identifiers (516-byte keys), not arbitrary strings
//  2. The onramp.Garlic.Listen() method does not accept address hints (it uses keystore)
//  3. Security: accepting arbitrary I2P addresses would require the corresponding private keys
//
// Implications:
//
//   - Multiple Listen calls will create multiple I2P destinations, each with a unique address
//   - To listen on a specific destination, configure the keystore before creating the Transport
//   - Callers expecting laddr to control the listening address will see unexpected behavior
//
// Upstream Fix:
//
//	To support user-provided laddr, onramp would need to either:
//	  1. Accept address hints and select matching keys from the keystore, or
//	  2. Support ephemeral destinations with runtime key generation
func (t *Transport) Listen(laddr ma.Multiaddr) (transport.Listener, error) {
	// Create the I2P listener
	netListener, err := t.garlic.Listen()
	if err != nil {
		return nil, fmt.Errorf("i2p: failed to create listener: %w", err)
	}

	// Get the I2P destination address from the garlic service
	// Use String() which returns the base32 address
	i2pAddr := t.garlic.String()
	if i2pAddr == "" {
		netListener.Close()
		return nil, fmt.Errorf("i2p: failed to get destination address")
	}

	// The address returned by garlic.String() should be in base32.b32.i2p format
	// Build the multiaddr: /garlic32/<base32-addr>.b32.i2p
	maddr, err := ma.NewMultiaddr(fmt.Sprintf("/garlic32/%s", i2pAddr))
	if err != nil {
		netListener.Close()
		return nil, fmt.Errorf("i2p: failed to create multiaddr: %w", err)
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

// Protocols returns the list of protocol codes this transport can handle.
// For I2P, this is just garlic32.
func (t *Transport) Protocols() []int {
	return []int{PGarlic32}
}

// Proxy returns true to indicate this is a proxy transport.
// I2P acts as an anonymizing proxy for all connections.
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

	// Close all active connections
	t.connMu.Lock()
	conns := make([]transport.CapableConn, 0, len(t.conns))
	for conn := range t.conns {
		conns = append(conns, conn)
	}
	t.connMu.Unlock()

	// Best-effort close of all connections
	for _, conn := range conns {
		conn.Close()
	}

	// Wait briefly for connections to close gracefully
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		t.connMu.RLock()
		remaining := len(t.conns)
		t.connMu.RUnlock()

		if remaining == 0 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Close the garlic service
	if t.garlic != nil {
		return t.garlic.Close()
	}

	return nil
}

// removeListener removes a listener from the transport's registry.
func (t *Transport) removeListener(l *listener) {
	t.listenersMu.Lock()
	delete(t.listeners, l.laddr.String())
	t.listenersMu.Unlock()
}
