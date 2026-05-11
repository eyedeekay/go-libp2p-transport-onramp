package tor

import (
	"testing"

	ma "github.com/multiformats/go-multiaddr"
)

// BenchmarkParseOnionMultiaddr benchmarks the parsing of onion multiaddrs
func BenchmarkParseOnionMultiaddr(b *testing.B) {
	// Valid onion3 address (56 chars = 32-byte pubkey + 2-byte checksum + 1-byte version)
	addr, _ := ma.NewMultiaddr("/onion3/vww6ybal4bd7szmgncyruucpgfkqahzddi37ktceo3ah7ngmcopnpyyd:80")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = parseOnionMultiaddr(addr)
	}
}

// BenchmarkIsOnionMultiaddr benchmarks the onion address detection
func BenchmarkIsOnionMultiaddr(b *testing.B) {
	onionAddr, _ := ma.NewMultiaddr("/onion3/vww6ybal4bd7szmgncyruucpgfkqahzddi37ktceo3ah7ngmcopnpyyd:80")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = isOnionMultiaddr(onionAddr)
	}
}

// BenchmarkIsOnionMultiaddr_NonOnion benchmarks detection on non-onion addresses
func BenchmarkIsOnionMultiaddr_NonOnion(b *testing.B) {
	ip4Addr, _ := ma.NewMultiaddr("/ip4/127.0.0.1/tcp/4001")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = isOnionMultiaddr(ip4Addr)
	}
}

// BenchmarkProtocolsConstant benchmarks the Protocols constant lookup
func BenchmarkProtocolsConstant(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = POnion3
	}
}
