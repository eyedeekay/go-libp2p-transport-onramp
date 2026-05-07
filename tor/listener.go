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
	// Accept the raw connection
	rawConn, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}

	// Wrap in manet.Conn
	maConn, err := manet.WrapNetConn(rawConn)
	if err != nil {
		rawConn.Close()
		return nil, fmt.Errorf("tor: failed to wrap connection: %w", err)
	}

	// Create a connection scope
	connScope, err := l.transport.rcmgr.OpenConnection(network.DirInbound, false, l.laddr)
	if err != nil {
		maConn.Close()
		return nil, fmt.Errorf("tor: failed to open connection scope: %w", err)
	}

	// Upgrade the connection
	// For inbound connections, we don't know the peer ID yet
	ctx := context.Background()
	conn, err := l.transport.upgrader.Upgrade(ctx, l.transport, maConn, network.DirInbound, "", connScope)
	if err != nil {
		connScope.Done()
		maConn.Close()
		return nil, fmt.Errorf("tor: upgrade failed: %w", err)
	}

	// Track the connection for proper cleanup on transport Close
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
