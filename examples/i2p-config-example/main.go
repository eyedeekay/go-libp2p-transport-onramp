// Package main demonstrates using the I2P transport with custom configuration.
package main

import (
	"fmt"
	"log"

	"github.com/go-i2p/go-libp2p-transport-onramp/i2p"
	"github.com/go-i2p/onramp"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/transport"
)

func main() {
	// Create custom I2P configuration
	// Example: Use a non-standard SAM address and higher tunnel count
	config := &i2p.TransportConfig{
		ServiceName: "my-custom-i2p-service",
		SAMAddr:     "127.0.0.1:7656", // Standard SAM address
		Options:     onramp.OPT_HUGE,  // Use more tunnels for higher performance
	}

	// Create libp2p host with I2P transport using custom configuration
	log.Println("Creating libp2p host with I2P transport (custom config)...")
	h, err := libp2p.New(
		libp2p.Transport(func(upgrader transport.Upgrader, rcmgr network.ResourceManager) (transport.Transport, error) {
			return i2p.NewTransportWithOptions(upgrader, rcmgr, config)
		}),
		libp2p.NoListenAddrs,
	)
	if err != nil {
		log.Fatalf("Failed to create libp2p host: %v", err)
	}
	defer h.Close()

	fmt.Printf("\n=== I2P Transport Initialized (Custom Config) ===\n")
	fmt.Printf("Service Name: %s\n", config.ServiceName)
	fmt.Printf("SAM Address: %s\n", config.SAMAddr)
	fmt.Printf("Options: OPT_HUGE (high performance)\n")
	fmt.Printf("Peer ID: %s\n", h.ID())
	fmt.Printf("\nThe host can now dial to garlic addresses.\n")
}
