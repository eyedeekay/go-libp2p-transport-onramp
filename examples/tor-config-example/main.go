// Package main demonstrates using the Tor transport with custom configuration.
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/go-i2p/go-libp2p-transport-onramp/tor"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/transport"
)

func main() {
	ctx := context.Background()

	// Create custom Tor configuration
	config := &tor.TransportConfig{
		ServiceName: "my-custom-libp2p-service",
	}

	// Create libp2p host with Tor transport using custom configuration
	log.Println("Creating libp2p host with Tor transport (custom config)...")
	h, err := libp2p.New(
		libp2p.Transport(func(upgrader transport.Upgrader, rcmgr network.ResourceManager) (transport.Transport, error) {
			return tor.NewTransportWithOptions(upgrader, rcmgr, config)
		}),
		libp2p.NoListenAddrs,
	)
	if err != nil {
		log.Fatalf("Failed to create libp2p host: %v", err)
	}
	defer h.Close()

	fmt.Printf("\n=== Tor Transport Initialized (Custom Config) ===\n")
	fmt.Printf("Service Name: %s\n", config.ServiceName)
	fmt.Printf("Peer ID: %s\n", h.ID())
	fmt.Printf("\nThe host can now dial to onion addresses.\n")

	_ = ctx
}
