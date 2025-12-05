// Package bridge provides the untrusted bridge for TEE I/O operations.
// This package handles communication between the enclave and the untrusted world.
package bridge

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

// SocketConfig holds socket bridge configuration.
type SocketConfig struct {
	Address     string
	ReadTimeout time.Duration
	WriteTimeout time.Duration
	MaxMessageSize int
}

// Socket provides raw I/O operations for the enclave.
// All data passing through this bridge should be encrypted.
type Socket struct {
	mu       sync.RWMutex
	config   SocketConfig
	listener net.Listener
	conns    map[string]net.Conn
	closed   bool
}

// NewSocket creates a new socket bridge.
func NewSocket(cfg SocketConfig) *Socket {
	if cfg.ReadTimeout == 0 {
		cfg.ReadTimeout = 30 * time.Second
	}
	if cfg.WriteTimeout == 0 {
		cfg.WriteTimeout = 30 * time.Second
	}
	if cfg.MaxMessageSize == 0 {
		cfg.MaxMessageSize = 1024 * 1024 // 1MB
	}

	return &Socket{
		config: cfg,
		conns:  make(map[string]net.Conn),
	}
}

// Listen starts listening for connections.
func (s *Socket) Listen(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return fmt.Errorf("socket is closed")
	}

	listener, err := net.Listen("tcp", s.config.Address)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	s.listener = listener
	return nil
}

// Accept accepts a new connection.
func (s *Socket) Accept(ctx context.Context) (string, error) {
	s.mu.RLock()
	listener := s.listener
	s.mu.RUnlock()

	if listener == nil {
		return "", fmt.Errorf("not listening")
	}

	conn, err := listener.Accept()
	if err != nil {
		return "", fmt.Errorf("accept: %w", err)
	}

	connID := fmt.Sprintf("%s-%d", conn.RemoteAddr().String(), time.Now().UnixNano())

	s.mu.Lock()
	s.conns[connID] = conn
	s.mu.Unlock()

	return connID, nil
}

// Connect connects to a remote address.
func (s *Socket) Connect(ctx context.Context, address string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return "", fmt.Errorf("socket is closed")
	}

	dialer := net.Dialer{
		Timeout: s.config.ReadTimeout,
	}

	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return "", fmt.Errorf("connect: %w", err)
	}

	connID := fmt.Sprintf("%s-%d", address, time.Now().UnixNano())
	s.conns[connID] = conn

	return connID, nil
}

// Read reads data from a connection.
// Data should be encrypted before leaving the enclave.
func (s *Socket) Read(ctx context.Context, connID string, size int) ([]byte, error) {
	s.mu.RLock()
	conn, ok := s.conns[connID]
	s.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("connection not found: %s", connID)
	}

	if size > s.config.MaxMessageSize {
		size = s.config.MaxMessageSize
	}

	if err := conn.SetReadDeadline(time.Now().Add(s.config.ReadTimeout)); err != nil {
		return nil, fmt.Errorf("set deadline: %w", err)
	}

	buf := make([]byte, size)
	n, err := conn.Read(buf)
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("read: %w", err)
	}

	return buf[:n], nil
}

// Write writes data to a connection.
// Data should be encrypted before leaving the enclave.
func (s *Socket) Write(ctx context.Context, connID string, data []byte) error {
	s.mu.RLock()
	conn, ok := s.conns[connID]
	s.mu.RUnlock()

	if !ok {
		return fmt.Errorf("connection not found: %s", connID)
	}

	if len(data) > s.config.MaxMessageSize {
		return fmt.Errorf("message too large: %d > %d", len(data), s.config.MaxMessageSize)
	}

	if err := conn.SetWriteDeadline(time.Now().Add(s.config.WriteTimeout)); err != nil {
		return fmt.Errorf("set deadline: %w", err)
	}

	_, err := conn.Write(data)
	if err != nil {
		return fmt.Errorf("write: %w", err)
	}

	return nil
}

// CloseConn closes a specific connection.
func (s *Socket) CloseConn(connID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	conn, ok := s.conns[connID]
	if !ok {
		return nil
	}

	delete(s.conns, connID)
	return conn.Close()
}

// Close closes the socket bridge.
func (s *Socket) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.closed = true

	// Close all connections
	for id, conn := range s.conns {
		conn.Close()
		delete(s.conns, id)
	}

	// Close listener
	if s.listener != nil {
		return s.listener.Close()
	}

	return nil
}

// Address returns the listening address.
func (s *Socket) Address() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.listener != nil {
		return s.listener.Addr().String()
	}
	return s.config.Address
}
