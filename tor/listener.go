// Package tor implements a libp2p transport for Tor's onion routing network.
//
// Code Duplication Note:
//
// This package shares significant logic with the i2p package (listener Accept flow,
// connection upgrade patterns, resource management). This duplication is INTENTIONAL
// to maintain package independence and clarity.
//
// Reasons for duplication:
//   - Tor and I2P are fundamentally different networks with distinct APIs and semantics
//   - Separate packages allow network-specific optimizations without cross-contamination
//   - Reduces coupling between tor and i2p implementations
//   - Makes each package independently understandable and testable
//   - Simplifies maintenance when one network requires changes the other doesn't
//
// Extracting shared logic to a common package would:
//   - Introduce coupling between tor and i2p
//   - Complicate network-specific customizations
//   - Make each package less self-contained
//   - Add indirection that obscures the control flow
//
// The duplication is acceptable given the modest codebase size (~850 LOC total) and
// the architectural benefits of package independence.
package tor

import (
	"context"
	"fmt"
	"net"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/transport"
	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
)

// listener wraps a net.Listener to implement transport.Listener.
type listener struct {
	net.Listener
	transport *Transport
	laddr     ma.Multiaddr
}

// Accept waits for and returns the next connection to the listener.
// The returned connection is upgraded with security and multiplexing.
func (l *listener) Accept() (transport.CapableConn, error) {
	// Step 1: Accept the raw TCP/SOCKS connection from the Tor daemon.
	// This gives us a net.Conn that carries encrypted Tor traffic, but it's not yet
	// integrated with libp2p's security and multiplexing layers.
	rawConn, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}

	// Step 2: Wrap the raw connection in a multiaddr-aware connection (manet.Conn).
	// This allows us to associate the connection with its multiaddr endpoints, which
	// libp2p uses for addressing and routing.
	maConn, err := manet.WrapNetConn(rawConn)
	if err != nil {
		rawConn.Close()
		return nil, fmt.Errorf("tor: failed to wrap connection: %w", err)
	}

	// Step 3: Create a resource manager scope for this connection.
	// The resource manager tracks and limits system resources (memory, file descriptors, etc.)
	// used by libp2p connections. We specify DirInbound to indicate this is an incoming connection.
	connScope, err := l.transport.rcmgr.OpenConnection(network.DirInbound, false, l.laddr)
	if err != nil {
		maConn.Close()
		return nil, fmt.Errorf("tor: failed to open connection scope: %w", err)
	}

	// Step 4: Upgrade the connection with security and multiplexing.
	// The upgrader performs:
	//   - Security handshake (noise or tls) to encrypt the connection and authenticate peers
	//   - Stream multiplexing (yamux or mplex) to allow multiple logical streams over one connection
	//   - Peer ID verification to ensure we're talking to the expected peer
	// For inbound connections, we don't know the peer ID in advance (empty string ""),
	// so the upgrader will extract it from the security handshake.
	ctx := context.Background()
	conn, err := l.transport.upgrader.Upgrade(ctx, l.transport, maConn, network.DirInbound, "", connScope)
	if err != nil {
		connScope.Done()
		maConn.Close()
		return nil, fmt.Errorf("tor: upgrade failed: %w", err)
	}

	// Step 5: Track the connection for proper cleanup.
	// When the transport is closed, we need to close all active connections.
	// The trackConnection wrapper ensures this connection is automatically unregistered
	// from the transport's connection registry when closed.
	wrappedConn := l.transport.trackConnection(conn)

	return wrappedConn, nil
}

// Close closes the listener.
// Any blocked Accept operations will be unblocked and return errors.
func (l *listener) Close() error {
	l.transport.removeListener(l)
	return l.Listener.Close()
}

// Addr returns the listener's network address.
func (l *listener) Addr() net.Addr {
	return l.Listener.Addr()
}

// Multiaddr returns the listener's multiaddr.
func (l *listener) Multiaddr() ma.Multiaddr {
	return l.laddr
}
