package tor

import (
	"fmt"
	"strings"

	ma "github.com/multiformats/go-multiaddr"
)

const (
	// P_ONION3 is the protocol code for onion3 addresses
	P_ONION3 = 445
)

// parseOnionMultiaddr extracts the onion address from a multiaddr.
// Expected format: /onion3/<56-char-base32>:<port> or /onion3/<56-char-base32>:<port>/...
// Returns the address in format "address.onion:port" for use with onramp.
func parseOnionMultiaddr(addr ma.Multiaddr) (string, error) {
	if addr == nil {
		return "", fmt.Errorf("multiaddr is nil")
	}

	// Split the multiaddr into components
	components := ma.Split(addr)

	// Find the onion3 component
	var onionValue string
	for _, comp := range components {
		protocols := comp.Protocols()
		if len(protocols) > 0 && protocols[0].Code == P_ONION3 {
			// Get the value for this component
			val, err := comp.ValueForProtocol(P_ONION3)
			if err != nil {
				return "", fmt.Errorf("failed to get onion3 value: %w", err)
			}
			onionValue = val
			break
		}
	}

	if onionValue == "" {
		return "", fmt.Errorf("no onion3 component found in multiaddr")
	}

	// The value should be in format "base32addr:port"
	// We need to convert it to "base32addr.onion:port"
	parts := strings.Split(onionValue, ":")
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid onion3 address format: expected addr:port, got %s", onionValue)
	}

	address := parts[0]
	port := parts[1]

	// Validate the address length (should be 56 characters for v3 onion)
	if len(address) != 56 {
		return "", fmt.Errorf("invalid onion3 address length: expected 56 chars, got %d", len(address))
	}

	// Return in the format expected by Tor and onramp
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
			if p.Code == P_ONION3 {
				return true
			}
		}
	}
	return false
}

// onionMultiaddrToString converts a multiaddr to a human-readable onion address string.
func onionMultiaddrToString(addr ma.Multiaddr) (string, error) {
	if !isOnionMultiaddr(addr) {
		return "", fmt.Errorf("not an onion multiaddr")
	}
	return parseOnionMultiaddr(addr)
}
