package tor

import (
	"context"
	"encoding/base32"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/cretz/bine/torutil/ed25519"
	"github.com/go-i2p/onramp"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/transport"
	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
	"golang.org/x/crypto/sha3"
)

// Transport implements the libp2p transport.Transport interface for Tor.
// It provides anonymous communication over the Tor network using onramp's Onion API.
type Transport struct {
	onion    *onramp.Onion
	upgrader transport.Upgrader
	rcmgr    network.ResourceManager

	listenersMu sync.RWMutex
	listeners   map[string]*listener

	connMu sync.RWMutex
	conns  map[transport.CapableConn]struct{}
}

// TransportConfig holds configuration options for the Tor transport.
type TransportConfig struct {
	// ServiceName is the identifier for the onion service.
	// If empty, defaults to "libp2p-tor".
	ServiceName string
}

// DefaultTransportConfig returns the default configuration for Tor transport.
func DefaultTransportConfig() *TransportConfig {
	return &TransportConfig{
		ServiceName: "libp2p-tor",
	}
}

// NewTransport creates a new Tor transport with default configuration.
// For custom configuration, use NewTransportWithOptions.
func NewTransport(upgrader transport.Upgrader, rcmgr network.ResourceManager) (*Transport, error) {
	return NewTransportWithOptions(upgrader, rcmgr, nil)
}

// NewTransportWithOptions creates a new Tor transport with custom configuration.
// If config is nil, default configuration is used.
func NewTransportWithOptions(upgrader transport.Upgrader, rcmgr network.ResourceManager, config *TransportConfig) (*Transport, error) {
	if upgrader == nil {
		return nil, fmt.Errorf("upgrader cannot be nil")
	}

	// Use default config if none provided
	if config == nil {
		config = DefaultTransportConfig()
	}

	// Set default service name if empty
	serviceName := config.ServiceName
	if serviceName == "" {
		serviceName = "libp2p-tor"
	}

	// Create onramp.Onion with configuration
	onion, err := onramp.NewOnion(serviceName)
	if err != nil {
		return nil, fmt.Errorf("tor: failed to create onion service: %w", err)
	}

	t := &Transport{
		onion:     onion,
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

// Dial dials a remote peer over Tor.
// The raddr must be an onion3 multiaddr (e.g., /onion3/<addr>:<port>).
//
// Context Cancellation Limitations:
//
// The underlying onramp.Onion.Dial method does not accept context.Context, so context
// cancellation is implemented via a goroutine-based workaround. This has trade-offs:
//
//   - If ctx is cancelled while Dial is in progress, this method returns immediately
//     with ctx.Err(), but the goroutine continues attempting to connect.
//
//   - If the goroutine later succeeds, the connection cannot be returned (caller is gone)
//     and will be leaked until garbage collection or the remote end closes it.
//
//   - Leaked connections consume file descriptors and may cause issues under high
//     cancellation rates (e.g., aggressive retry loops with short timeouts).
//
// Mitigation:
//   - Use reasonable context timeouts (≥30s for Tor to avoid frequent leaks)
//   - Monitor open file descriptors in production
//
// Upstream Fix:
//
//	To eliminate leaks, onramp.Onion.Dial would need to either:
//	  1. Accept a context.Context parameter, or
//	  2. Return a cancellable operation handle
//
// See: https://github.com/go-i2p/onramp/issues/XXX (TODO: file issue upstream)
func (t *Transport) Dial(ctx context.Context, raddr ma.Multiaddr, p peer.ID) (transport.CapableConn, error) {
	// Parse the onion multiaddr
	onionAddr, err := parseOnionMultiaddr(raddr)
	if err != nil {
		return nil, fmt.Errorf("tor: invalid multiaddr: %w", err)
	}

	// Dial using onramp with context cancellation support.
	// Since onramp.Onion.Dial doesn't support context, we wrap it in a goroutine
	// and use a select statement to respect context cancellation.
	type dialResult struct {
		conn net.Conn
		err  error
	}
	resultChan := make(chan dialResult, 1)

	go func() {
		conn, err := t.onion.Dial("tcp", onionAddr)
		resultChan <- dialResult{conn, err}
	}()

	var rawConn net.Conn
	select {
	case <-ctx.Done():
		// Context cancelled before dial completed. The goroutine will continue
		// executing and may eventually succeed, but we cannot retrieve or close
		// the connection from here (no handle to the in-flight dial operation).
		// This is an unavoidable limitation without context support in onramp.
		return nil, fmt.Errorf("tor: dial cancelled: %w", ctx.Err())
	case result := <-resultChan:
		if result.err != nil {
			return nil, fmt.Errorf("tor: dial failed: %w", result.err)
		}
		rawConn = result.conn
	}

	// Wrap in manet.Conn
	// For Tor outbound connections, the local address is the SOCKS proxy address,
	// which manet.WrapNetConn will automatically retrieve from the raw socket's LocalAddr().
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

	// Track the connection for proper cleanup on transport Close
	wrappedConn := t.trackConnection(conn)

	return wrappedConn, nil
}

// CanDial returns true if this transport can dial the given multiaddr.
// It returns true only for onion3 multiaddrs.
// Returns false for all other address types including IP addresses (ip4, ip6),
// DNS addresses, or other protocol multiaddrs that are not onion3.
func (t *Transport) CanDial(addr ma.Multiaddr) bool {
	return isOnionMultiaddr(addr)
}

// Protocols returns the list of protocols this transport handles.
// For Tor, this is just the onion3 protocol.
func (t *Transport) Protocols() []int {
	return []int{POnion3}
}

// Listen listens for incoming connections on the given multiaddr.
// For Tor, this creates an onion service.
//
// laddr Parameter Behavior:
//
// The laddr parameter is IGNORED by this implementation. The onion address is always
// auto-generated by onramp based on persistent cryptographic keys stored in the keystore.
//
// This differs from typical transport.Transport behavior where laddr specifies the
// listening address:
//
//   - In typical transports (TCP, QUIC), laddr determines the bind address (e.g., 0.0.0.0:4001)
//   - In Tor, the onion address is derived from the public key and cannot be chosen
//
// Why laddr Cannot Be Used:
//
//  1. Onion addresses are cryptographic identifiers derived from ed25519 keys, not arbitrary strings
//  2. The onramp.Onion.Listen() method does not accept address hints (it reads from keystore)
//  3. Security: accepting arbitrary onion addresses would require the corresponding private keys
//
// Implications:
//
//   - Multiple Listen calls will create multiple onion services, each with a unique auto-generated address
//   - To listen on a specific onion address, you must configure the keystore before creating the Transport
//   - Callers expecting laddr to control the listening address will see unexpected behavior
//
// Upstream Fix:
//
//	To support user-provided laddr, onramp would need to either:
//	  1. Accept address hints and select matching keys from the keystore, or
//	  2. Support ephemeral onion services with runtime key generation
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
	if idx := strings.LastIndex(listenerAddr, ":"); idx >= 0 {
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

// getOnionAddress derives the v3 onion address from ed25519 keys.
// The v3 onion address format is:
// base32(version || pubkey || checksum) where:
//   - version is 0x03 (1 byte)
//   - pubkey is the 32-byte ed25519 public key
//   - checksum is the first 2 bytes of SHA3-256(".onion checksum" || pubkey || version)
func getOnionAddress(keys ed25519.KeyPair) string {
	if keys == nil {
		return "unknown"
	}

	// Get the public key
	pubKey := keys.PublicKey()
	if len(pubKey) != 32 {
		return "invalid-pubkey-length"
	}

	// Version byte for v3 onion addresses (defined in tor-spec.txt)
	version := byte(0x03)

	// Calculate checksum using SHA3-256 (SHAKE-256 variant as specified in tor-spec).
	// The checksum prevents address typos and ensures address integrity.
	// Format: SHA3-256(".onion checksum" || pubkey || version)
	// We concatenate:
	//   1. The literal string ".onion checksum" (15 bytes)
	//   2. The 32-byte ed25519 public key
	//   3. The 1-byte version (0x03)
	// Total input: 48 bytes
	checksumInput := append([]byte(".onion checksum"), pubKey...)
	checksumInput = append(checksumInput, version)

	// Hash the input and take the first 2 bytes as the checksum
	hash := sha3.New256()
	hash.Write(checksumInput)
	checksum := hash.Sum(nil)[:2] // First 2 bytes of 32-byte hash

	// Build the address: version || pubkey || checksum
	// Total: 1 + 32 + 2 = 35 bytes before base32 encoding
	addressBytes := make([]byte, 0, 35)
	addressBytes = append(addressBytes, version)
	addressBytes = append(addressBytes, pubKey...)
	addressBytes = append(addressBytes, checksum...)

	// Encode to base32 (lowercase, no padding)
	encoder := base32.StdEncoding.WithPadding(base32.NoPadding)
	encoded := encoder.EncodeToString(addressBytes)

	// Convert to lowercase (Tor uses lowercase for v3 addresses)
	// Convert to lowercase (base32 addresses are case-insensitive, lowercase is standard)
	return strings.ToLower(encoded)
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
