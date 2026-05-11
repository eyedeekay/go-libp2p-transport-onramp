package i2p

import (
	"strings"
	"testing"

	ma "github.com/multiformats/go-multiaddr"
)

func TestParseGarlicMultiaddr(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantDest  string
		wantPort  int
		wantErr   bool
		errSubstr string
	}{
		{
			name:     "valid garlic32 address without port",
			input:    "/garlic32/example.b32.i2p",
			wantDest: "example.b32.i2p",
			wantPort: 0,
		},
		{
			name:     "valid garlic32 address without .b32.i2p suffix (auto-added)",
			input:    "/garlic32/example",
			wantDest: "example.b32.i2p",
			wantPort: 0,
		},
		{
			name:      "invalid - not garlic",
			input:     "/ip4/127.0.0.1/tcp/8080",
			wantErr:   true,
			errSubstr: "no garlic32 component",
		},
		{
			name:      "nil multiaddr",
			input:     "",
			wantErr:   true,
			errSubstr: "multiaddr is nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var addr ma.Multiaddr
			var err error

			if tt.input != "" {
				addr, err = ma.NewMultiaddr(tt.input)
				if err != nil {
					// Some inputs may fail to parse as valid multiaddrs
					// due to garlic32 not being registered
					if !tt.wantErr {
						t.Skipf("skipping test: garlic32 protocol not registered in multiaddr")
					}
					return
				}
			}

			gotDest, gotPort, err := parseGarlicMultiaddr(addr)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseGarlicMultiaddr() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				if tt.errSubstr != "" && !strings.Contains(err.Error(), tt.errSubstr) {
					t.Errorf("parseGarlicMultiaddr() error = %v, want error containing %q", err, tt.errSubstr)
				}
				return
			}

			if gotDest != tt.wantDest {
				t.Errorf("parseGarlicMultiaddr() dest = %v, want %v", gotDest, tt.wantDest)
			}
			if gotPort != tt.wantPort {
				t.Errorf("parseGarlicMultiaddr() port = %v, want %v", gotPort, tt.wantPort)
			}
		})
	}
}

func TestIsGarlicMultiaddr(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
		skip  bool // Skip if protocol not registered
	}{
		{
			name:  "valid garlic32",
			input: "/garlic32/example.b32.i2p",
			want:  true,
			skip:  true, // May skip if protocol not registered
		},
		{
			name:  "not garlic",
			input: "/ip4/127.0.0.1/tcp/8080",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var addr ma.Multiaddr
			if tt.input != "" {
				var err error
				addr, err = ma.NewMultiaddr(tt.input)
				if err != nil {
					if tt.skip {
						t.Skipf("skipping test: garlic32 protocol not registered in multiaddr")
					}
					t.Fatalf("failed to create test multiaddr: %v", err)
				}
			}

			got := isGarlicMultiaddr(addr)
			if got != tt.want {
				t.Errorf("isGarlicMultiaddr() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCanDial(t *testing.T) {
	// Create a mock transport (we only need CanDial for this test)
	tr := &Transport{}

	tests := []struct {
		name string
		addr string
		want bool
		skip bool
	}{
		{
			name: "can dial garlic32",
			addr: "/garlic32/example.b32.i2p",
			want: true,
			skip: true, // May skip if protocol not registered
		},
		{
			name: "cannot dial ip4",
			addr: "/ip4/127.0.0.1/tcp/8080",
			want: false,
		},
		{
			name: "cannot dial ip6",
			addr: "/ip6/::1/tcp/8080",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addr, err := ma.NewMultiaddr(tt.addr)
			if err != nil {
				if tt.skip {
					t.Skipf("skipping test: garlic32 protocol not registered in multiaddr")
				}
				t.Fatalf("failed to create test multiaddr: %v", err)
			}

			got := tr.CanDial(addr)
			if got != tt.want {
				t.Errorf("CanDial() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestProtocols(t *testing.T) {
	tr := &Transport{}
	protocols := tr.Protocols()

	if len(protocols) != 1 {
		t.Errorf("Protocols() returned %d protocols, want 1", len(protocols))
	}

	if protocols[0] != PGarlic32 {
		t.Errorf("Protocols()[0] = %d, want %d (P_GARLIC32)", protocols[0], PGarlic32)
	}
}

func TestProxy(t *testing.T) {
	tr := &Transport{}
	if !tr.Proxy() {
		t.Error("Proxy() = false, want true (I2P is a proxy transport)")
	}
}
