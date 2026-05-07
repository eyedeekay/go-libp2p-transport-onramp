# go-libp2p-transport-onramp

[![CI](https://github.com/go-i2p/go-libp2p-transport-onramp/actions/workflows/ci.yml/badge.svg)](https://github.com/go-i2p/go-libp2p-transport-onramp/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/go-i2p/go-libp2p-transport-onramp)](https://goreportcard.com/report/github.com/go-i2p/go-libp2p-transport-onramp)
[![GoDoc](https://pkg.go.dev/badge/github.com/go-i2p/go-libp2p-transport-onramp)](https://pkg.go.dev/github.com/go-i2p/go-libp2p-transport-onramp)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

libp2p transports written around the go-i2p/onramp API

## Overview

This repository provides two libp2p transport implementations:
- **Tor Transport** - Uses onramp's Onion API for anonymous communication over Tor
- **I2P Transport** - Uses onramp's Garlic API for anonymous communication over I2P

These transports integrate seamlessly with libp2p's networking stack, enabling privacy-preserving peer-to-peer applications.

## Status

**✅ Implementation Complete** - Core transports, unit tests, integration tests, and examples all working.

### Completed
- ✅ go.mod with all dependencies (232 entries in go.sum)
- ✅ Tor transport core (addr parsing, Dial, Listen, interface methods)
- ✅ I2P transport core (addr parsing, Dial, Listen, interface methods)
- ✅ Unit tests for address parsing and transport methods
- ✅ Integration tests with external Tor/I2P daemons
- ✅ Connection wrapping for libp2p upgrader compatibility
- ✅ Resource management integration
- ✅ Example programs demonstrating usage
- ✅ Proper v3 onion address generation from ed25519 keys (SHA3-256 checksum)

### Known Limitations
- 🔄 Garlic32 protocol (code 456) not officially registered in multiaddr - some tests skip when creating garlic32 addresses

### Future Enhancements
- ⬜ Register garlic32 protocol in go-multiaddr upstream
- ⬜ Enhanced error handling and logging
- ⬜ Performance benchmarks
- ✅ CI/CD setup (GitHub Actions with test/lint/build workflows, Dependabot for dependency updates)

## Features (Planned)

- ✅ Full `transport.Transport` interface compliance
- ✅ Native multiaddr support (`/onion3/...` and `/garlic32/...` or `/garlic64/...`)
- ✅ Delegates protocol complexity to go-i2p/onramp
- ✅ Stream multiplexing via libp2p (yamux/mplex)
- ✅ Security via libp2p (noise/tls)
- ✅ Peer authentication using libp2p peer IDs
- ✅ Connection lifecycle management
- ✅ I2P virtual port support (FROM_PORT/TO_PORT)

## Quick Start (After Implementation)

### Installation

```bash
go get github.com/go-i2p/go-libp2p-transport-onramp
```

### Running Tests

**Unit tests** (no external dependencies):
```bash
go test ./...
```

**Integration tests** (requires Tor daemon on port 9050 and I2P router with SAM on port 7656):
```bash
go test -v -tags=integration ./...
```

### Examples

See [examples/](examples/) directory for working examples:
- [examples/tor-example/](examples/tor-example/) - Basic Tor transport initialization
- [examples/tor-config-example/](examples/tor-config-example/) - Tor transport with custom configuration
- [examples/i2p-example/](examples/i2p-example/) - Basic I2P transport initialization
- [examples/i2p-config-example/](examples/i2p-config-example/) - I2P transport with custom configuration

Run examples:
```bash
go run ./examples/tor-example         # Requires Tor daemon
go run ./examples/tor-config-example  # Requires Tor daemon
go run ./examples/i2p-example         # Requires I2P router with SAM
go run ./examples/i2p-config-example  # Requires I2P router with SAM
```

## Configuration

Both transports support custom configuration through `NewTransportWithOptions` functions while maintaining backward compatibility with the original `NewTransport` API.

### Tor Configuration

```go
config := &tor.TransportConfig{
    ServiceName: "my-custom-libp2p-service", // Custom onion service name (default: "libp2p-tor")
}

transport, err := tor.NewTransportWithOptions(upgrader, rcmgr, config)
```

### I2P Configuration

```go
config := &i2p.TransportConfig{
    ServiceName: "my-custom-i2p-service",   // Custom I2P tunnel name (default: "libp2p-i2p")
    SAMAddr:     "127.0.0.1:7656",          // SAM bridge address (default: "127.0.0.1:7656")
    Options:     onramp.OPT_HUGE,           // Tunnel options (default: onramp.OPT_DEFAULTS)
}

transport, err := i2p.NewTransportWithOptions(upgrader, rcmgr, config)
```

**Tunnel Options:**
- `onramp.OPT_DEFAULTS` - Balanced performance (2 inbound, 2 outbound tunnels)
- `onramp.OPT_HUGE` - High performance (6 inbound, 6 outbound tunnels)
- Custom options as []string (see [onramp documentation](https://pkg.go.dev/github.com/go-i2p/onramp))

## Usage

### Tor Transport

```go
import (
    "github.com/libp2p/go-libp2p"
    "github.com/libp2p/go-libp2p/core/network"
    "github.com/libp2p/go-libp2p/core/transport"
    tortransport "github.com/go-i2p/go-libp2p-transport-onramp/tor"
)

func main() {
    host, err := libp2p.New(
        libp2p.Transport(func(upgrader transport.Upgrader, rcmgr network.ResourceManager) (transport.Transport, error) {
            return tortransport.NewTransport(upgrader, rcmgr)
        }),
        libp2p.NoListenAddrs,
    )
    if err != nil {
        log.Fatal(err)
    }
    defer host.Close()
    
    // Host can now dial to /onion3/ addresses
}
```

### I2P Transport

```go
import (
    "github.com/libp2p/go-libp2p"
    "github.com/libp2p/go-libp2p/core/network"
    "github.com/libp2p/go-libp2p/core/transport"
    i2ptransport "github.com/go-i2p/go-libp2p-transport-onramp/i2p"
)

func main() {
    host, err := libp2p.New(
        libp2p.Transport(func(upgrader transport.Upgrader, rcmgr network.ResourceManager) (transport.Transport, error) {
            return i2ptransport.NewTransport(upgrader, rcmgr)
        }),
        libp2p.NoListenAddrs,
    )
    if err != nil {
        log.Fatal(err)
    }
    defer host.Close()
    
    // Host can now dial to /garlic32/ addresses
}
```

## Documentation

- [Implementation Plan](IMPLEMENTATION_PLAN.md) - Detailed design and development roadmap
- Examples - Coming soon in `examples/` directory

## Requirements

- Go 1.21 or later
- For Tor: Running Tor daemon (or use onramp's embedded Tor)
- For I2P: Running I2P router with SAM bridge enabled (or use onramp's embedded SAM)

## Dependencies

- [go-libp2p](https://github.com/libp2p/go-libp2p) - libp2p implementation in Go
- [go-i2p/onramp](https://github.com/go-i2p/onramp) - High-level I2P and Tor API
- [go-multiaddr](https://github.com/multiformats/go-multiaddr) - Multiaddr format

## Contributing

This project is in the planning phase. Contributions and feedback on the implementation plan are welcome!

## License

See [LICENSE](LICENSE) file.
