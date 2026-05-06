//go:build integration
// +build integration

package tor_test

import (
	"bufio"
	"context"
	"io"
	"testing"
	"time"

	"github.com/go-i2p/go-libp2p-transport-onramp/tor"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/libp2p/go-libp2p/core/transport"
)

const testProtocol = protocol.ID("/test/echo/1.0.0")

// TestTorDialAndStream tests creating two Tor-enabled hosts,
// having one listen and the other dial and send data.
//
// Prerequisites: Tor daemon running on localhost:9050
func TestTorDialAndStream(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Create listener host
	listener, err := libp2p.New(
		libp2p.Transport(func(upgrader transport.Upgrader, rcmgr network.ResourceManager) (transport.Transport, error) {
			return tor.NewTransport(upgrader, rcmgr)
		}),
		libp2p.NoListenAddrs,
	)
	if err != nil {
		t.Fatalf("Failed to create listener host: %v", err)
	}
	defer listener.Close()

	// Set up echo handler
	listener.SetStreamHandler(testProtocol, func(s network.Stream) {
		defer s.Close()
		_, err := io.Copy(s, s) // Echo back
		if err != nil {
			t.Logf("Echo handler error: %v", err)
		}
	})

	// Get Tor transport and create listener
	torTransport := getTransport(t, listener)
	torListener, err := torTransport.Listen(nil)
	if err != nil {
		t.Fatalf("Failed to create Tor listener: %v", err)
	}
	defer torListener.Close()

	listenerAddr := torListener.Multiaddr()
	t.Logf("Listening on: %s", listenerAddr)

	// Create dialer host
	dialer, err := libp2p.New(
		libp2p.Transport(func(upgrader transport.Upgrader, rcmgr network.ResourceManager) (transport.Transport, error) {
			return tor.NewTransport(upgrader, rcmgr)
		}),
		libp2p.NoListenAddrs,
	)
	if err != nil {
		t.Fatalf("Failed to create dialer host: %v", err)
	}
	defer dialer.Close()

	// Add listener's peer info
	dialer.Peerstore().AddAddrs(listener.ID(), []network.Multiaddr{listenerAddr}, time.Hour)

	// Connect and open stream
	t.Logf("Dialing from %s to %s", dialer.ID(), listener.ID())
	
	stream, err := dialer.NewStream(ctx, listener.ID(), testProtocol)
	if err != nil {
		t.Fatalf("Failed to open stream: %v", err)
	}
	defer stream.Close()

	// Send test message
	testMsg := "Hello over Tor!\n"
	writer := bufio.NewWriter(stream)
	_, err = writer.WriteString(testMsg)
	if err != nil {
		t.Fatalf("Failed to write: %v", err)
	}
	err = writer.Flush()
	if err != nil {
		t.Fatalf("Failed to flush: %v", err)
	}

	// Read echo response
	reader := bufio.NewReader(stream)
	response, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("Failed to read: %v", err)
	}

	if response != testMsg {
		t.Errorf("Expected %q, got %q", testMsg, response)
	}

	t.Logf("Successfully echoed message over Tor!")
}

// TestTorTransportProperties verifies the Tor transport reports correct properties
func TestTorTransportProperties(t *testing.T) {
	h, err := libp2p.New(
		libp2p.Transport(func(upgrader transport.Upgrader, rcmgr network.ResourceManager) (transport.Transport, error) {
			return tor.NewTransport(upgrader, rcmgr)
		}),
		libp2p.NoListenAddrs,
	)
	if err != nil {
		t.Fatalf("Failed to create host: %v", err)
	}
	defer h.Close()

	torTransport := getTransport(t, h)

	// Check protocols
	protocols := torTransport.Protocols()
	if len(protocols) != 1 || protocols[0] != 445 { // P_ONION3
		t.Errorf("Expected protocols [445], got %v", protocols)
	}

	// Check proxy flag
	if !torTransport.Proxy() {
		t.Error("Expected Proxy() to return true")
	}

	t.Logf("Transport properties verified")
}

// getTransport extracts the Tor transport from a host
func getTransport(t *testing.T, h host.Host) *tor.Transport {
	t.Helper()
	
	// Access the network's transports
	swarm, ok := h.Network().(interface {
		Transports() []transport.Transport
	})
	if !ok {
		t.Fatal("Network does not expose Transports()")
	}

	for _, tr := range swarm.Transports() {
		if torTr, ok := tr.(*tor.Transport); ok {
			return torTr
		}
	}

	t.Fatal("Tor transport not found in host")
	return nil
}
