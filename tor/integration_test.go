//go:build integration
// +build integration

package tor_test

import (
"context"
"testing"
"time"

"github.com/go-i2p/go-libp2p-transport-onramp/tor"
"github.com/libp2p/go-libp2p"
"github.com/libp2p/go-libp2p/core/network"
"github.com/libp2p/go-libp2p/core/transport"
)

// TestTorTransportIntegration verifies the Tor transport can be created
// and integrated with a libp2p host successfully.
//
// Prerequisites: Tor daemon running on localhost:9050
func TestTorTransportIntegration(t *testing.T) {
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

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

_ = ctx
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
