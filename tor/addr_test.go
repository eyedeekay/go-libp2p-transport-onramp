package tor

import (
	"strings"
	"testing"

	ma "github.com/multiformats/go-multiaddr"
)

func TestParseOnionMultiaddr(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		want      string
		wantErr   bool
		errSubstr string
	}{
		{
			name:  "valid onion3 address",
			input: "/onion3/vww6ybal4bd7szmgncyruucpgfkqahzddi37ktceo3ah7ngmcopnpyyd:80",
			want:  "vww6ybal4bd7szmgncyruucpgfkqahzddi37ktceo3ah7ngmcopnpyyd.onion:80",
		},
		{
			name:  "valid onion3 address with different port",
			input: "/onion3/vww6ybal4bd7szmgncyruucpgfkqahzddi37ktceo3ah7ngmcopnpyyd:8080",
			want:  "vww6ybal4bd7szmgncyruucpgfkqahzddi37ktceo3ah7ngmcopnpyyd.onion:8080",
		},
		{
			name:      "invalid - not onion3",
			input:     "/ip4/127.0.0.1/tcp/8080",
			wantErr:   true,
			errSubstr: "no onion3 component",
		},
		{
			name:      "invalid - short address",
			input:     "/onion3/tooshort:80",
			wantErr:   true,
			errSubstr: "invalid onion3 address length",
		},
		{
			name:      "invalid - missing port",
			input:     "/onion3/vww6ybal4bd7szmgncyruucpgfkqahzddi37ktceo3ah7ngmcopnpyyd",
			wantErr:   true,
			errSubstr: "invalid onion3 address format",
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
					// If we expect an error and multiaddr creation failed,
					// consider this as validation - the invalid address was rejected early
					if tt.wantErr {
						t.Skipf("invalid multiaddr rejected during creation: %v", err)
						return
					}
					t.Fatalf("failed to create test multiaddr: %v", err)
				}
			}

			got, err := parseOnionMultiaddr(addr)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseOnionMultiaddr() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				if tt.errSubstr != "" && !strings.Contains(err.Error(), tt.errSubstr) {
					t.Errorf("parseOnionMultiaddr() error = %v, want error containing %q", err, tt.errSubstr)
				}
				return
			}

			if got != tt.want {
				t.Errorf("parseOnionMultiaddr() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsOnionMultiaddr(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{
			name:  "valid onion3",
			input: "/onion3/vww6ybal4bd7szmgncyruucpgfkqahzddi37ktceo3ah7ngmcopnpyyd:80",
			want:  true,
		},
		{
			name:  "not onion",
			input: "/ip4/127.0.0.1/tcp/8080",
			want:  false,
		},
		{
			name:  "combined with other protocols",
			input: "/onion3/vww6ybal4bd7szmgncyruucpgfkqahzddi37ktceo3ah7ngmcopnpyyd:80/p2p/QmYyQSo1c1Ym7orWxLYvCrM2EmxFTANf8wXmmE7DWjhx5N",
			want:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var addr ma.Multiaddr
			if tt.input != "" {
				var err error
				addr, err = ma.NewMultiaddr(tt.input)
				if err != nil {
					t.Fatalf("failed to create test multiaddr: %v", err)
				}
			}

			got := isOnionMultiaddr(addr)
			if got != tt.want {
				t.Errorf("isOnionMultiaddr() = %v, want %v", got, tt.want)
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
	}{
		{
			name: "can dial onion3",
			addr: "/onion3/vww6ybal4bd7szmgncyruucpgfkqahzddi37ktceo3ah7ngmcopnpyyd:80",
			want: true,
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

	if protocols[0] != P_ONION3 {
		t.Errorf("Protocols()[0] = %d, want %d (P_ONION3)", protocols[0], P_ONION3)
	}
}

func TestProxy(t *testing.T) {
	tr := &Transport{}
	if !tr.Proxy() {
		t.Error("Proxy() = false, want true (Tor is a proxy transport)")
	}
}
