//go:build integration
// +build integration

package i2p_test

import (
	"bufio"
	"context"
	"io"
	"testing"
	"time"

	"github.com/go-i2p/go-libp2p-transport-onramp/i2p"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/libp2p/go-libp2p/core/transport"
	ma "github.com/multiformats/go-multiaddr"
)

const testProtocol = protocol.ID("/test/echo/1.0.0")

// TestI2PDialAndStream tests creating two I2P-enabled hosts,
// having one listen and the other dial and send data.
//
// Prerequisites: I2P router running with SAM bridge on localhost:7656
func TestI2PDialAndStream(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	// Create listener host
	listener, err := libp2p.New(
		libp2p.Transport(func(upgrader transport.Upgrader, rcmgr network.ResourceManager) (transport.Transport, error) {
			return i2p.NewTransport(upgrader, rcmgr)
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

	// Get I2P transport and create listener
	i2pTransport := getTransport(t, listener)
	i2pListener, err := i2pTransport.Listen(nil)
	if err != nil {
		t.Fatalf("Failed to create I2P listener: %v", err)
	}
	defer i2pListener.Close()

	listenerAddr := i2pListener.Multiaddr()
	t.Logf("Listening on: %s", listenerAddr)

	// Create dialer host
	dialer, err := libp2p.New(
		libp2p.Transport(func(upgrader transport.Upgrader, rcmgr network.ResourceManager) (transport.Transport, error) {
			return i2p.NewTransport(upgrader, rcmgr)
		}),
		libp2p.NoListenAddrs,
	)
	if err != nil {
		t.Fatalf("Failed to create dialer host: %v", err)
	}
	defer dialer.Close()

	// Add listener's peer info
	dialer.Peerstore().AddAddrs(listener.ID(), []ma.Multiaddr{listenerAddr}, time.Hour)

	// Connect and open stream
	t.Logf("Dialing from %s to %s", dialer.ID(), listener.ID())

	stream, err := dialer.NewStream(ctx, listener.ID(), testProtocol)
	if err != nil {
		t.Fatalf("Failed to open stream: %v", err)
	}
	defer stream.Close()

	// Send test message
	testMsg := "Hello over I2P!\n"
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

	t.Logf("Successfully echoed message over I2P!")
}

// TestI2PTransportProperties verifies the I2P transport reports correct properties
func TestI2PTransportProperties(t *testing.T) {
	h, err := libp2p.New(
		libp2p.Transport(func(upgrader transport.Upgrader, rcmgr network.ResourceManager) (transport.Transport, error) {
			return i2p.NewTransport(upgrader, rcmgr)
		}),
		libp2p.NoListenAddrs,
	)
	if err != nil {
		t.Fatalf("Failed to create host: %v", err)
	}
	defer h.Close()

	i2pTransport := getTransport(t, h)

	// Check protocols
	protocols := i2pTransport.Protocols()
	if len(protocols) != 1 || protocols[0] != 456 { // P_GARLIC32
		t.Errorf("Expected protocols [456], got %v", protocols)
	}

	// Check proxy flag
	if !i2pTransport.Proxy() {
		t.Error("Expected Proxy() to return true")
	}

	t.Logf("Transport properties verified")
}

// getTransport extracts the I2P transport from a host
func getTransport(t *testing.T, h host.Host) *i2p.Transport {
	t.Helper()

	// Access the network's transports
	swarm, ok := h.Network().(interface {
		Transports() []transport.Transport
	})
	if !ok {
		t.Fatal("Network does not expose Transports()")
	}

	for _, tr := range swarm.Transports() {
		if i2pTr, ok := tr.(*i2p.Transport); ok {
			return i2pTr
		}
	}

	t.Fatal("I2P transport not found in host")
	return nil
}
