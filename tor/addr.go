package tor

import (
	"fmt"
	"strings"

	ma "github.com/multiformats/go-multiaddr"
)

const (
	// POnion3 is the protocol code for onion3 addresses
	POnion3 = 445
)

// parseOnionMultiaddr extracts the onion address from a multiaddr.
// Expected format: /onion3/<56-char-base32>:<port> or /onion3/<56-char-base32>:<port>/...
// Returns the address in format "address.onion:port" for use with onramp.
func parseOnionMultiaddr(addr ma.Multiaddr) (string, error) {
	if addr == nil {
		return "", fmt.Errorf("multiaddr is nil")
	}

	// Split the multiaddr into components. A multiaddr like /onion3/abc:123/tcp/456
	// becomes separate components: [/onion3/abc:123, /tcp/456]
	components := ma.Split(addr)

	// Find the onion3 component by iterating through all components.
	// We must iterate because multiaddr can contain multiple protocols (e.g., /onion3/.../tcp/...)
	// and we need to find the specific onion3 protocol component.
	var onionValue string
	for _, comp := range components {
		// Each component may have multiple protocols, but typically has one.
		// We check if the first protocol in this component is onion3 (code 445).
		protocols := comp.Protocols()
		if len(protocols) > 0 && protocols[0].Code == POnion3 {
			// Extract the value associated with the onion3 protocol.
			// For /onion3/abc:123, this returns "abc:123"
			val, err := comp.ValueForProtocol(POnion3)
			if err != nil {
				return "", fmt.Errorf("tor: failed to get onion3 value: %w", err)
			}
			onionValue = val
			break
		}
	}

	if onionValue == "" {
		return "", fmt.Errorf("no onion3 component found in multiaddr")
	}

	// The onion3 protocol stores addresses in "base32addr:port" format.
	// Split on ":" to separate the base32 address from the port number.
	parts := strings.Split(onionValue, ":")
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid onion3 address format: expected addr:port, got %s", onionValue)
	}

	address := parts[0]
	port := parts[1]

	// Validate the address length. v3 onion addresses are always 56 base32 characters
	// (derived from 32-byte ed25519 public key + 2-byte checksum + 1-byte version).
	if len(address) != 56 {
		return "", fmt.Errorf("invalid onion3 address length: expected 56 chars, got %d", len(address))
	}

	// Convert to the format expected by Tor SOCKS proxy and onramp: "address.onion:port"
	return fmt.Sprintf("%s.onion:%s", address, port), nil
}

// isOnionMultiaddr checks if the multiaddr contains an onion3 protocol.
func isOnionMultiaddr(addr ma.Multiaddr) bool {
	if addr == nil {
		return false
	}

	components := ma.Split(addr)
	for _, comp := range components {
		protocols := comp.Protocols()
		for _, p := range protocols {
			if p.Code == POnion3 {
				return true
			}
		}
	}
	return false
}
