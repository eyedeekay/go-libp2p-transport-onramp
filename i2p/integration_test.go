//go:build integration
// +build integration

package i2p_test

import (
	"context"
	"testing"
	"time"

	"github.com/go-i2p/go-libp2p-transport-onramp/i2p"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/transport"
)

// TestI2PTransportIntegration verifies the I2P transport can be created
// and integrated with a libp2p host successfully.
//
// Prerequisites: I2P router running with SAM bridge on localhost:7656
func TestI2PTransportIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

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

	_ = ctx
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
