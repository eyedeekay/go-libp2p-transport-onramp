package tor

import (
	"net"

	"github.com/cretz/bine/torutil/ed25519"
)

// mockOnion is a mock implementation of onramp.Onion for testing
type mockOnion struct {
	dialFunc   func(network, addr string) (net.Conn, error)
	listenFunc func(args ...string) (net.Listener, error)
	keysFunc   func() (ed25519.KeyPair, error)
	closeFunc  func() error
}

func (m *mockOnion) Dial(network, addr string) (net.Conn, error) {
	if m.dialFunc != nil {
		return m.dialFunc(network, addr)
	}
	return nil, nil
}

func (m *mockOnion) Listen(args ...string) (net.Listener, error) {
	if m.listenFunc != nil {
		return m.listenFunc(args...)
	}
	return nil, nil
}

func (m *mockOnion) Keys() (ed25519.KeyPair, error) {
	if m.keysFunc != nil {
		return m.keysFunc()
	}
	// Return a valid key pair for testing
	keyPair, _ := ed25519.GenerateKey(nil)
	return keyPair, nil
}

func (m *mockOnion) Close() error {
	if m.closeFunc != nil {
		return m.closeFunc()
	}
	return nil
}

// mockAddr implements net.Addr for testing
type mockAddr struct {
	network string
	addr    string
}

func (m mockAddr) Network() string { return m.network }
func (m mockAddr) String() string  { return m.addr }

// mockConn is a mock implementation of net.Conn for testing
type mockConn struct {
	net.Conn
	readFunc       func([]byte) (int, error)
	writeFunc      func([]byte) (int, error)
	closeFunc      func() error
	localAddrFunc  func() net.Addr
	remoteAddrFunc func() net.Addr
}

func (m *mockConn) Read(b []byte) (int, error) {
	if m.readFunc != nil {
		return m.readFunc(b)
	}
	return 0, nil
}

func (m *mockConn) Write(b []byte) (int, error) {
	if m.writeFunc != nil {
		return m.writeFunc(b)
	}
	return len(b), nil
}

func (m *mockConn) Close() error {
	if m.closeFunc != nil {
		return m.closeFunc()
	}
	return nil
}

func (m *mockConn) LocalAddr() net.Addr {
	if m.localAddrFunc != nil {
		return m.localAddrFunc()
	}
	return mockAddr{network: "tcp", addr: "127.0.0.1:9050"}
}

func (m *mockConn) RemoteAddr() net.Addr {
	if m.remoteAddrFunc != nil {
		return m.remoteAddrFunc()
	}
	return mockAddr{network: "tcp", addr: "example.onion:80"}
}

// mockListener is a mock implementation of net.Listener for testing
type mockListener struct {
	acceptFunc func() (net.Conn, error)
	closeFunc  func() error
	addrFunc   func() net.Addr
}

func (m *mockListener) Accept() (net.Conn, error) {
	if m.acceptFunc != nil {
		return m.acceptFunc()
	}
	return &mockConn{}, nil
}

func (m *mockListener) Close() error {
	if m.closeFunc != nil {
		return m.closeFunc()
	}
	return nil
}

func (m *mockListener) Addr() net.Addr {
	if m.addrFunc != nil {
		return m.addrFunc()
	}
	return mockAddr{network: "tcp", addr: "0.0.0.0:0"}
}
