package transport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"net"
	"os"
	"sync"
	"syscall"
	"time"
)

var (
	clientDialTimeout    = 5 * time.Second
	clientWriteTimeout   = 5 * time.Second
	staleDialTimeout     = 1 * time.Second
	serverAcceptTimeout  = 1 * time.Second
	serverReadTimeout    = 30 * time.Second
	clientRetryBackoffs  = []time.Duration{100 * time.Millisecond, 200 * time.Millisecond, 400 * time.Millisecond}
	clientRetryJitterMax = 25 * time.Millisecond
)

// MessageHandler handles one decoded JSON payload from a UDS connection.
type MessageHandler func(data []byte)

// Server serves incoming Unix Domain Socket connections.
type Server struct {
	listener   net.Listener
	handler    MessageHandler
	socketPath string
	mu         sync.Mutex
}

// NewServer creates a new UDS server.
func NewServer(socketPath string, handler MessageHandler) *Server {
	return &Server{
		handler:    handler,
		socketPath: socketPath,
	}
}

// Start starts the UDS server and accept loop.
func (s *Server) Start(ctx context.Context) error {
	if err := s.cleanupStaleSocket(); err != nil {
		return fmt.Errorf("transport.Server.Start: %w", err)
	}

	listener, err := net.Listen("unix", s.socketPath)
	if err != nil {
		return fmt.Errorf("transport.Server.Start: %w", err)
	}

	s.mu.Lock()
	s.listener = listener
	s.mu.Unlock()

	go func() {
		<-ctx.Done()
		if err := s.Stop(); err != nil {
			slog.Error("failed to stop UDS server", "socket_path", s.socketPath, "error", err)
		}
	}()

	go s.acceptLoop(ctx)

	return nil
}

// Stop stops the UDS server and removes its socket file.
func (s *Server) Stop() error {
	s.mu.Lock()
	listener := s.listener
	s.mu.Unlock()

	if listener != nil {
		if err := listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			return fmt.Errorf("transport.Server.Stop: %w", err)
		}
	}

	s.mu.Lock()
	s.listener = nil
	s.mu.Unlock()

	if err := os.Remove(s.socketPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("transport.Server.Stop: %w", err)
	}

	return nil
}

func (s *Server) cleanupStaleSocket() error {
	conn, err := net.DialTimeout("unix", s.socketPath, staleDialTimeout)
	if err == nil {
		_ = conn.Close()
		return fmt.Errorf("socket already active: %s", s.socketPath)
	}

	if errors.Is(err, syscall.ENOENT) {
		return nil
	}

	if !errors.Is(err, syscall.ECONNREFUSED) {
		_, statErr := os.Stat(s.socketPath)
		if statErr == nil {
			return fmt.Errorf("dial existing socket: %w", err)
		}
		if errors.Is(statErr, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("stat socket path: %w", statErr)
	}

	if err := os.Remove(s.socketPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove stale socket: %w", err)
	}

	return nil
}

func (s *Server) acceptLoop(ctx context.Context) {
	for {
		listener := s.currentListener()
		if listener == nil {
			return
		}
		if err := setAcceptDeadline(listener, time.Now().Add(serverAcceptTimeout)); err != nil {
			slog.Error("failed to set accept deadline", "socket_path", s.socketPath, "error", err)
			return
		}

		conn, err := listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) || ctx.Err() != nil {
				return
			}
			if isTimeoutError(err) {
				continue
			}
			slog.Error("failed to accept UDS connection", "socket_path", s.socketPath, "error", err)
			continue
		}

		go s.handleConnection(ctx, conn)
	}
}

func (s *Server) currentListener() net.Listener {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.listener
}

func (s *Server) handleConnection(ctx context.Context, conn net.Conn) {
	defer conn.Close()

	decoder := json.NewDecoder(conn)
	for {
		if ctx.Err() != nil {
			return
		}
		if err := conn.SetReadDeadline(time.Now().Add(serverReadTimeout)); err != nil {
			slog.Error("failed to set read deadline", "socket_path", s.socketPath, "error", err)
			return
		}

		var payload json.RawMessage
		if err := decoder.Decode(&payload); err != nil {
			if errors.Is(err, io.EOF) {
				return
			}
			if isTimeoutError(err) {
				return
			}
			slog.Error("failed to decode UDS payload", "socket_path", s.socketPath, "error", err)
			return
		}

		if s.handler != nil {
			s.handler(payload)
		}
	}
}

// Client sends payloads to UDS servers.
type Client struct{}

// NewClient creates a new UDS client.
func NewClient() *Client {
	return &Client{}
}

func (c *Client) Send(ctx context.Context, socketPath string, data []byte) error {
	if err := c.sendOnce(ctx, socketPath, data); err == nil {
		return nil
	} else {
		lastErr := err
		for attempt, backoff := range clientRetryBackoffs {
			if ctx.Err() != nil {
				return fmt.Errorf("transport.Client.Send: %w", ctx.Err())
			}
			delay := backoff + retryJitter()
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				if !timer.Stop() {
					<-timer.C
				}
				return fmt.Errorf("transport.Client.Send: %w", ctx.Err())
			case <-timer.C:
			}
			if err := c.sendOnce(ctx, socketPath, data); err != nil {
				lastErr = err
				slog.Debug("retrying UDS send", "socket_path", socketPath, "attempt", attempt+2, "error", err)
				continue
			}
			return nil
		}
		return fmt.Errorf("transport.Client.Send: %w", lastErr)
	}
}

func (c *Client) sendOnce(ctx context.Context, socketPath string, data []byte) error {
	conn, err := net.DialTimeout("unix", socketPath, clientDialTimeout)
	if err != nil {
		return fmt.Errorf("dial socket: %w", err)
	}
	defer conn.Close()

	deadline := time.Now().Add(clientWriteTimeout)
	if dl, ok := ctx.Deadline(); ok && dl.Before(deadline) {
		deadline = dl
	}
	if err := conn.SetWriteDeadline(deadline); err != nil {
		return fmt.Errorf("set write deadline: %w", err)
	}

	payload := append([]byte(nil), data...)
	payload = append(payload, '\n')
	if _, err := conn.Write(payload); err != nil {
		return fmt.Errorf("write payload: %w", err)
	}

	return nil
}

func setAcceptDeadline(listener net.Listener, deadline time.Time) error {
	deadlineSetter, ok := listener.(interface{ SetDeadline(time.Time) error })
	if !ok {
		return nil
	}
	return deadlineSetter.SetDeadline(deadline)
}

func isTimeoutError(err error) bool {
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

func retryJitter() time.Duration {
	if clientRetryJitterMax <= 0 {
		return 0
	}
	return time.Duration(rand.Int63n(int64(clientRetryJitterMax)))
}
