package i2p

import (
	"testing"

	ma "github.com/multiformats/go-multiaddr"
)

// BenchmarkParseGarlicMultiaddr benchmarks the parsing of garlic multiaddrs
func BenchmarkParseGarlicMultiaddr(b *testing.B) {
	// Valid garlic32 address
	addr, _ := ma.NewMultiaddr("/garlic32/566niximlxdzpanmn4qouucvua3k7neniwss47li5r6ugoertzuq.b32.i2p")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = parseGarlicMultiaddr(addr)
	}
}

// BenchmarkIsGarlicMultiaddr benchmarks the garlic address detection
func BenchmarkIsGarlicMultiaddr(b *testing.B) {
	garlicAddr, _ := ma.NewMultiaddr("/garlic32/566niximlxdzpanmn4qouucvua3k7neniwss47li5r6ugoertzuq.b32.i2p")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = isGarlicMultiaddr(garlicAddr)
	}
}

// BenchmarkIsGarlicMultiaddr_NonGarlic benchmarks detection on non-garlic addresses
func BenchmarkIsGarlicMultiaddr_NonGarlic(b *testing.B) {
	ip4Addr, _ := ma.NewMultiaddr("/ip4/127.0.0.1/tcp/4001")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = isGarlicMultiaddr(ip4Addr)
	}
}

// BenchmarkProtocolsConstant benchmarks the Protocols constant lookup
func BenchmarkProtocolsConstant(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = PGarlic32
	}
}
