# go-libp2p-transport-onramp

libp2p transports written around the go-i2p/onramp API

## Overview

This repository provides two libp2p transport implementations:
- **Tor Transport** - Uses onramp's Onion API for anonymous communication over Tor
- **I2P Transport** - Uses onramp's Garlic API for anonymous communication over I2P

These transports integrate seamlessly with libp2p's networking stack, enabling privacy-preserving peer-to-peer applications.

## Status

**Implementation Phase** - Core transports implemented and unit tests passing.

### Completed
- ✅ go.mod with all dependencies
- ✅ Tor transport core (addr parsing, Dial, Listen, interface methods)
- ✅ I2P transport core (addr parsing, Dial, Listen, interface methods)
- ✅ Unit tests for address parsing and transport methods
- ✅ Connection wrapping for libp2p upgrader compatibility
- ✅ Resource management integration

### In Progress
- 🔄 Onion address derivation from ed25519 keys (placeholder implementation)
- 🔄 Garlic32 protocol registration in multiaddr (custom code 456)

### Remaining
- ⬜ Integration tests with external Tor/I2P daemons
- ⬜ Example programs (echo servers/clients)
- ⬜ Documentation (godoc comments)
- ⬜ CI/CD setup

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

### Tor Transport

```go
import (
    "github.com/libp2p/go-libp2p"
    tortransport "github.com/go-i2p/go-libp2p-transport-onramp/tor"
)

func main() {
    torTpt, err := tortransport.NewTransport()
    if err != nil {
        log.Fatal(err)
    }
    
    host, err := libp2p.New(
        libp2p.Transport(torTpt),
        libp2p.ListenAddrStrings("/onion3/..."),
    )
    if err != nil {
        log.Fatal(err)
    }
    defer host.Close()
}
```

### I2P Transport

```go
import (
    "github.com/libp2p/go-libp2p"
    i2ptransport "github.com/go-i2p/go-libp2p-transport-onramp/i2p"
)

func main() {
    i2pTpt, err := i2ptransport.NewTransport()
    if err != nil {
        log.Fatal(err)
    }
    
    host, err := libp2p.New(
        libp2p.Transport(i2pTpt),
    )
    if err != nil {
        log.Fatal(err)
    }
    defer host.Close()
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
