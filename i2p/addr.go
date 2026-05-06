package i2p

import (
	"fmt"
	"strings"

	ma "github.com/multiformats/go-multiaddr"
)

// Note: garlic32 and garlic64 protocols may not be officially registered in go-multiaddr.
// We'll need to register them or use a workaround. For now, we'll define custom protocol codes.
const (
	// P_GARLIC32 is the protocol code for garlic32 addresses (base32 I2P addresses)
	// This is a custom protocol code that should be registered with multiaddr
	P_GARLIC32 = 456 // Placeholder - needs official registration or local registration
)

// parseGarlicMultiaddr extracts the I2P destination from a multiaddr.
// Expected format: /garlic32/<base32-addr>.b32.i2p or /garlic32/<base32-addr>.b32.i2p/tcp/<port>
// Returns the address in a format usable by onramp (e.g., "address.b32.i2p" or "address.b32.i2p:port").
func parseGarlicMultiaddr(addr ma.Multiaddr) (string, int, error) {
	if addr == nil {
		return "", 0, fmt.Errorf("multiaddr is nil")
	}

	// Split the multiaddr into components
	components := ma.Split(addr)

	// Find the garlic32 component
	var garlicValue string
	for _, comp := range components {
		protocols := comp.Protocols()
		if len(protocols) > 0 && protocols[0].Code == P_GARLIC32 {
			// Get the value for this component
			val, err := comp.ValueForProtocol(P_GARLIC32)
			if err != nil {
				return "", 0, fmt.Errorf("failed to get garlic32 value: %w", err)
			}
			garlicValue = val
			break
		}
	}

	if garlicValue == "" {
		return "", 0, fmt.Errorf("no garlic32 component found in multiaddr")
	}

	// Get the value of the garlic32 address
	value := garlicValue

	// Ensure the address has the .b32.i2p suffix
	if !strings.HasSuffix(value, ".b32.i2p") {
		value = value + ".b32.i2p"
	}

	// Check for a port in the multiaddr (looking for tcp component)
	port := 0
	for _, comp := range components {
		protocols := comp.Protocols()
		for _, p := range protocols {
			if p.Name == "tcp" {
				portStr, err := comp.ValueForProtocol(p.Code)
				if err == nil {
					fmt.Sscanf(portStr, "%d", &port)
				}
				break
			}
		}
	}

	return value, port, nil
}

// isGarlicMultiaddr checks if the multiaddr contains a garlic32 protocol.
func isGarlicMultiaddr(addr ma.Multiaddr) bool {
	if addr == nil {
		return false
	}

	components := ma.Split(addr)
	for _, comp := range components {
		protocols := comp.Protocols()
		for _, p := range protocols {
			if p.Code == P_GARLIC32 {
				return true
			}
		}
	}
	return false
}

// garlicMultiaddrToString converts a multiaddr to a human-readable garlic address string.
func garlicMultiaddrToString(addr ma.Multiaddr) (string, error) {
	if !isGarlicMultiaddr(addr) {
		return "", fmt.Errorf("not a garlic multiaddr")
	}

	dest, port, err := parseGarlicMultiaddr(addr)
	if err != nil {
		return "", err
	}

	if port > 0 {
		return fmt.Sprintf("%s:%d", dest, port), nil
	}
	return dest, nil
}

// registerGarlicProtocols attempts to check if garlic32 protocol is registered.
//
// Note: go-multiaddr v0.16.1 does not provide a public API to register custom
// protocols at runtime. The garlic32 protocol (code 456) is used internally by
// this transport but may not be recognized by multiaddr's parsing functions.
//
// This means:
//   - Dial() and Listen() work correctly (they parse addresses internally)
//   - NewMultiaddr("/garlic32/...") may fail if the protocol isn't registered
//   - Some tests skip when multiaddr cannot create garlic32 addresses
//
// To use garlic32 addresses:
//  1. Use the transport's internal parsing (this works out of the box)
//  2. Submit a PR to add garlic32 to go-multiaddr's protocol list
//  3. Use a forked version of go-multiaddr with garlic32 added
//
// The protocol specification:
//   - Name: "garlic32"
//   - Code: 456
//   - Size: -1 (variable length, format: <base32>.b32.i2p)
func registerGarlicProtocols() error {
	// Check if garlic32 is already registered
	protocols, err := ma.ProtocolsWithString("garlic32")
	if err == nil && len(protocols) > 0 {
		// Already registered - great!
		return nil
	}

	// Protocol not registered. This is expected with standard go-multiaddr.
	// The transport will still work, but some multiaddr operations may fail.
	return fmt.Errorf("garlic32 protocol (code 456) not registered in go-multiaddr")
}

func init() {
	// Check protocol registration status
	// This is informational only - the transport works regardless
	_ = registerGarlicProtocols()
}
