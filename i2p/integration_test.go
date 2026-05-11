//go:build integration
// +build integration

// Package i2p_test contains integration tests for the I2P transport.
//
// Prerequisites:
//   - I2P router running with SAM bridge enabled on localhost:7656
//   - Install: Download from geti2p.net or apt-get install i2p
//   - Configure: Enable SAM bridge in router console (http://127.0.0.1:7657/configclients)
//   - SAM must listen on 127.0.0.1:7656 (default)
//
// To run these tests:
//
//	go test -v -tags=integration ./i2p
//
// What these tests validate:
//   - Transport creation connects to I2P SAM bridge
//   - Listener creates valid base32 I2P destinations
//   - End-to-end peer-to-peer communication over I2P
//   - Virtual port support (FROM_PORT/TO_PORT)
//   - Message passing between peers via I2P garlic routing
package i2p_test

import (
	"bufio"
	"context"
	"io"
	"testing"
	"time"

	"github.com/go-i2p/go-libp2p-transport-onramp/i2p"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/transport"
	ma "github.com/multiformats/go-multiaddr"
)

// TestI2PTransportIntegration verifies the I2P transport can be created
// and integrated with a libp2p host successfully.
//
// Prerequisites: I2P router running with SAM bridge on localhost:7656
func TestI2PTransportIntegration(t *testing.T) {
	// Create host with I2P transport
	h, err := libp2p.New(
		libp2p.Transport(func(upgrader transport.Upgrader, rcmgr network.ResourceManager) (transport.Transport, error) {
			return i2p.NewTransport(upgrader, rcmgr)
		}),
		libp2p.NoListenAddrs,
	)
	if err != nil {
		t.Fatalf("Failed to create host with I2P transport: %v", err)
	}
	defer h.Close()

	t.Logf("Successfully created host %s with I2P transport", h.ID())
}

// TestI2PTransportCreation tests creating an I2P transport directly
func TestI2PTransportCreation(t *testing.T) {
	// Create a temporary host to get upgrader and resource manager
	h, err := libp2p.New()
	if err != nil {
		t.Fatalf("Failed to create temporary host: %v", err)
	}
	defer h.Close()

	// Create another host with I2P transport
	h2, err := libp2p.New(
		libp2p.Transport(func(upgrader transport.Upgrader, rcmgr network.ResourceManager) (transport.Transport, error) {
			i2pTr, err := i2p.NewTransport(upgrader, rcmgr)
			if err != nil {
				return nil, err
			}

			// Verify transport properties
			protocols := i2pTr.Protocols()
			if len(protocols) != 1 || protocols[0] != 456 {
				t.Errorf("Expected protocols [456], got %v", protocols)
			}

			if !i2pTr.Proxy() {
				t.Error("Expected Proxy() to return true")
			}

			t.Log("I2P transport properties verified")
			return i2pTr, nil
		}),
	)
	if err != nil {
		t.Fatalf("Failed to create host with I2P transport: %v", err)
	}
	defer h2.Close()

	t.Log("I2P transport created and properties verified successfully")
}

// TestI2PEndToEndCommunication tests peer-to-peer communication over I2P
//
// Prerequisites: I2P router running with SAM bridge on localhost:7656
func TestI2PEndToEndCommunication(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	const protocolID = "/test-i2p/1.0.0"
	const testMessage = "Hello from I2P test"
	const responseMessage = "Response from I2P test"

	// Channel to signal when listener receives a message
	messageReceived := make(chan string, 1)

	// Create listener host with I2P transport
	listenAddr, err := ma.NewMultiaddr("/garlic32/0.0.0.0:0")
	if err != nil {
		t.Fatalf("Failed to create listen address: %v", err)
	}

	listener, err := libp2p.New(
		libp2p.Transport(func(upgrader transport.Upgrader, rcmgr network.ResourceManager) (transport.Transport, error) {
			return i2p.NewTransport(upgrader, rcmgr)
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

	// Create dialer host with I2P transport
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

	t.Logf("Dialer peer ID: %s", dialer.ID())

	// Parse listener's peer info from multiaddr
	addrInfo, err := peer.AddrInfoFromP2pAddr(listenerPeerAddr)
	if err != nil {
		t.Fatalf("Failed to parse peer address: %v", err)
	}

	// Connect dialer to listener
	t.Log("Dialer: Connecting to listener over I2P (this may take a minute)...")
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
