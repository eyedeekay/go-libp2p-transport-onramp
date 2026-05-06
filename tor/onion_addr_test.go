package tor

import (
	"strings"
	"testing"

	"github.com/cretz/bine/torutil/ed25519"
)

// TestGetOnionAddress verifies that onion address generation produces
// valid v3 onion addresses from ed25519 keys
func TestGetOnionAddress(t *testing.T) {
	// Create a test keypair
	keyPair, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	// Generate onion address
	addr := getOnionAddress(keyPair)

	// Validate the address format
	t.Run("address_length", func(t *testing.T) {
		// v3 onion addresses are 56 characters long
		if len(addr) != 56 {
			t.Errorf("Expected address length 56, got %d: %s", len(addr), addr)
		}
	})

	t.Run("address_format", func(t *testing.T) {
		// Should be all lowercase alphanumeric (base32)
		for _, c := range addr {
			if !((c >= 'a' && c <= 'z') || (c >= '2' && c <= '7')) {
				t.Errorf("Invalid character %c in address %s (expected lowercase base32)", c, addr)
			}
		}
	})

	t.Run("address_consistency", func(t *testing.T) {
		// Same key should produce same address
		addr2 := getOnionAddress(keyPair)
		if addr != addr2 {
			t.Errorf("Same keypair produced different addresses: %s vs %s", addr, addr2)
		}
	})

	t.Run("different_keys_different_addresses", func(t *testing.T) {
		// Different keys should produce different addresses
		keyPair2, err := ed25519.GenerateKey(nil)
		if err != nil {
			t.Fatalf("Failed to generate second key: %v", err)
		}
		addr2 := getOnionAddress(keyPair2)
		if addr == addr2 {
			t.Errorf("Different keypairs produced same address: %s", addr)
		}
	})

	t.Run("nil_keypair", func(t *testing.T) {
		// Nil keypair should return "unknown"
		addr := getOnionAddress(nil)
		if addr != "unknown" {
			t.Errorf("Expected 'unknown' for nil keypair, got %s", addr)
		}
	})

	t.Logf("Generated valid v3 onion address: %s.onion", addr)
}

// TestToLowerBase32 verifies the base32 lowercase conversion
func TestToLowerBase32(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "all_uppercase",
			input:    "ABCDEFG234567",
			expected: "abcdefg234567",
		},
		{
			name:     "mixed_case",
			input:    "AbCdEfG234567",
			expected: "abcdefg234567",
		},
		{
			name:     "already_lowercase",
			input:    "abcdefg234567",
			expected: "abcdefg234567",
		},
		{
			name:     "numbers_only",
			input:    "234567",
			expected: "234567",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toLowerBase32(tt.input)
			if result != tt.expected {
				t.Errorf("toLowerBase32(%s) = %s, want %s", tt.input, result, tt.expected)
			}
		})
	}
}

// TestOnionAddressRoundtrip verifies that generated addresses can be used
func TestOnionAddressRoundtrip(t *testing.T) {
	// Create a test keypair
	keyPair, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	// Generate onion address
	addr := getOnionAddress(keyPair)

	// Verify it can be formatted as a proper .onion address
	fullAddr := addr + ".onion"

	if !strings.HasSuffix(fullAddr, ".onion") {
		t.Errorf("Address doesn't end with .onion: %s", fullAddr)
	}

	if len(strings.TrimSuffix(fullAddr, ".onion")) != 56 {
		t.Errorf("Base address is not 56 characters: %s", fullAddr)
	}

	t.Logf("Full onion address: %s", fullAddr)
}
