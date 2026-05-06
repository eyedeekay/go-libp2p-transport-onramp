// Package main demonstrates using the I2P transport with libp2p.
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/go-i2p/go-libp2p-transport-onramp/i2p"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/transport"
)

func main() {
	ctx := context.Background()

	// Create libp2p host with I2P transport
	log.Println("Creating libp2p host with I2P transport...")
	h, err := libp2p.New(
		libp2p.Transport(func(upgrader transport.Upgrader, rcmgr network.ResourceManager) (transport.Transport, error) {
			return i2p.NewTransport(upgrader, rcmgr)
		}),
		libp2p.NoListenAddrs,
	)
	if err != nil {
		log.Fatalf("Failed to create libp2p host: %v", err)
	}
	defer h.Close()

	fmt.Printf("\n=== I2P Transport Initialized ===\n")
	fmt.Printf("Peer ID: %s\n", h.ID())
	fmt.Printf("\nThe host can now dial to garlic addresses.\n")

	_ = ctx
}
