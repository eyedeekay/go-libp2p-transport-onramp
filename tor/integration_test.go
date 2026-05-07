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
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/transport"
	ma "github.com/multiformats/go-multiaddr"
)

// TestTorTransportIntegration verifies the Tor transport can be created
// and integrated with a libp2p host successfully.
//
// Prerequisites: Tor daemon running on localhost:9050
func TestTorTransportIntegration(t *testing.T) {
	// Create host with Tor transport
	h, err := libp2p.New(
		libp2p.Transport(func(upgrader transport.Upgrader, rcmgr network.ResourceManager) (transport.Transport, error) {
			return tor.NewTransport(upgrader, rcmgr)
		}),
		libp2p.NoListenAddrs,
	)
	if err != nil {
		t.Fatalf("Failed to create host with Tor transport: %v", err)
	}
	defer h.Close()

	t.Logf("Successfully created host %s with Tor transport", h.ID())
}

// TestTorTransportCreation tests creating a Tor transport directly
func TestTorTransportCreation(t *testing.T) {
	// Create a temporary host to get upgrader and resource manager
	h, err := libp2p.New()
	if err != nil {
		t.Fatalf("Failed to create temporary host: %v", err)
	}
	defer h.Close()

	// Create another host with Tor transport
	h2, err := libp2p.New(
		libp2p.Transport(func(upgrader transport.Upgrader, rcmgr network.ResourceManager) (transport.Transport, error) {
			torTr, err := tor.NewTransport(upgrader, rcmgr)
			if err != nil {
				return nil, err
			}

			// Verify transport properties
			protocols := torTr.Protocols()
			if len(protocols) != 1 || protocols[0] != 445 {
				t.Errorf("Expected protocols [445], got %v", protocols)
			}

			if !torTr.Proxy() {
				t.Error("Expected Proxy() to return true")
			}

			t.Log("Tor transport properties verified")
			return torTr, nil
		}),
	)
	if err != nil {
		t.Fatalf("Failed to create host with Tor transport: %v", err)
	}
	defer h2.Close()

	t.Log("Tor transport created and properties verified successfully")
}

// TestTorEndToEndCommunication tests peer-to-peer communication over Tor
//
// Prerequisites: Tor daemon running on localhost:9050
func TestTorEndToEndCommunication(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	const protocolID = "/test-tor/1.0.0"
	const testMessage = "Hello from Tor test"
	const responseMessage = "Response from Tor test"

	// Channel to signal when listener receives a message
	messageReceived := make(chan string, 1)

	// Create listener host with Tor transport
	listenAddr, err := ma.NewMultiaddr("/onion3/0.0.0.0:0")
	if err != nil {
		t.Fatalf("Failed to create listen address: %v", err)
	}

	listener, err := libp2p.New(
		libp2p.Transport(func(upgrader transport.Upgrader, rcmgr network.ResourceManager) (transport.Transport, error) {
			return tor.NewTransport(upgrader, rcmgr)
		}),
		libp2p.ListenAddrs(listenAddr),
	)
	if err != nil {
		t.Fatalf("Failed to create listener host: %v", err)
	}
	defer listener.Close()

	// Set up stream handler on listener
	listener.SetStreamHandler(protocolID, func(s network.Stream) {
		defer s.Close()
		t.Logf("Listener: Received connection from peer %s", s.Conn().RemotePeer())

		// Read message
		reader := bufio.NewReader(s)
		msg, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			t.Errorf("Listener: Failed to read message: %v", err)
			return
		}
		t.Logf("Listener: Received message: %s", msg)
		messageReceived <- msg

		// Send response
		if _, err := s.Write([]byte(responseMessage + "\n")); err != nil {
			t.Errorf("Listener: Failed to write response: %v", err)
		}
		t.Log("Listener: Sent response")
	})

	// Get listener's addresses
	addrs := listener.Addrs()
	if len(addrs) == 0 {
		t.Fatal("Listener has no addresses")
	}
	t.Logf("Listener ready at: %s", addrs[0])
	listenerPeerAddr := addrs[0].Encapsulate(ma.StringCast("/p2p/" + listener.ID().String()))
	t.Logf("Listener full address: %s", listenerPeerAddr)

	// Create dialer host with Tor transport
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

	t.Logf("Dialer peer ID: %s", dialer.ID())

	// Parse listener's peer info from multiaddr
	addrInfo, err := peer.AddrInfoFromP2pAddr(listenerPeerAddr)
	if err != nil {
		t.Fatalf("Failed to parse peer address: %v", err)
	}

	// Connect dialer to listener
	t.Log("Dialer: Connecting to listener over Tor (this may take a minute)...")
	connectCtx, connectCancel := context.WithTimeout(ctx, 90*time.Second)
	defer connectCancel()

	if err := dialer.Connect(connectCtx, *addrInfo); err != nil {
		t.Fatalf("Dialer: Failed to connect: %v", err)
	}
	t.Log("Dialer: Connected successfully")

	// Open stream from dialer to listener
	stream, err := dialer.NewStream(ctx, listener.ID(), protocolID)
	if err != nil {
		t.Fatalf("Dialer: Failed to open stream: %v", err)
	}
	defer stream.Close()

	t.Log("Dialer: Stream opened")

	// Send message
	if _, err := stream.Write([]byte(testMessage + "\n")); err != nil {
		t.Fatalf("Dialer: Failed to send message: %v", err)
	}
	t.Logf("Dialer: Sent message: %s", testMessage)

	// Read response
	reader := bufio.NewReader(stream)
	response, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		t.Fatalf("Dialer: Failed to read response: %v", err)
	}
	t.Logf("Dialer: Received response: %s", response)

	// Wait for listener to receive message
	select {
	case msg := <-messageReceived:
		if msg != testMessage+"\n" {
			t.Errorf("Message mismatch: expected %q, got %q", testMessage+"\n", msg)
		}
	case <-time.After(5 * time.Second):
		t.Error("Timeout waiting for listener to receive message")
	}

	// Verify connection properties
	conns := dialer.Network().ConnsToPeer(listener.ID())
	if len(conns) == 0 {
		t.Fatal("No connections found between peers")
	}

	conn := conns[0]
	t.Logf("Connection established:")
	t.Logf("  Local peer: %s", conn.LocalPeer())
	t.Logf("  Remote peer: %s", conn.RemotePeer())
	t.Logf("  Local addr: %s", conn.LocalMultiaddr())
	t.Logf("  Remote addr: %s", conn.RemoteMultiaddr())

	t.Log("End-to-end communication test passed!")
}
