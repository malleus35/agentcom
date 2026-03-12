package transport

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/peanut-cc/agentcom/internal/db"
)

func setupTransportTestDB(t *testing.T) *db.DB {
	t.Helper()

	database, err := db.OpenMemory()
	if err != nil {
		t.Fatalf("db.OpenMemory() error = %v", err)
	}
	t.Cleanup(func() {
		if err := database.Close(); err != nil {
			t.Fatalf("database.Close() error = %v", err)
		}
	})
	if err := database.Migrate(context.Background()); err != nil {
		t.Fatalf("database.Migrate() error = %v", err)
	}

	return database
}

func setupSocketPath(t *testing.T, name string) string {
	t.Helper()

	dir, err := os.MkdirTemp("/tmp", "agentcom-transport-")
	if err != nil {
		t.Fatalf("os.MkdirTemp() error = %v", err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(dir)
	})

	return filepath.Join(dir, name)
}

func TestServerClientRoundTrip(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	socketPath := setupSocketPath(t, "agent.sock")
	received := make(chan []byte, 1)

	server := NewServer(socketPath, func(data []byte) {
		received <- append([]byte(nil), data...)
	})
	if err := server.Start(ctx); err != nil {
		t.Fatalf("Server.Start() error = %v", err)
	}
	t.Cleanup(func() {
		_ = server.Stop()
	})

	client := NewClient()
	payload := []byte(`{"type":"ping"}`)
	if err := client.Send(context.Background(), socketPath, payload); err != nil {
		t.Fatalf("Client.Send() error = %v", err)
	}

	select {
	case got := <-received:
		if string(got) != string(payload) {
			t.Fatalf("received payload = %q, want %q", string(got), string(payload))
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for UDS payload")
	}
}

func TestServerStartRemovesStaleSocket(t *testing.T) {
	socketPath := setupSocketPath(t, "stale.sock")
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("net.Listen() error = %v", err)
	}
	if err := listener.Close(); err != nil {
		t.Fatalf("listener.Close() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	server := NewServer(socketPath, nil)
	if err := server.Start(ctx); err != nil {
		t.Fatalf("Server.Start() with stale socket error = %v", err)
	}
	defer server.Stop()
}

func TestClientSendCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := NewClient().Send(ctx, filepath.Join(t.TempDir(), "missing.sock"), []byte(`{"type":"ping"}`))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Client.Send(canceled) error = %v, want %v", err, context.Canceled)
	}
}

func TestPollerDeliversUnreadMessages(t *testing.T) {
	database := setupTransportTestDB(t)
	ctx := context.Background()

	sender := &db.Agent{Name: "sender", Type: "worker", Status: "alive"}
	if err := database.InsertAgent(ctx, sender); err != nil {
		t.Fatalf("InsertAgent(sender) error = %v", err)
	}
	receiver := &db.Agent{Name: "receiver", Type: "worker", Status: "alive"}
	if err := database.InsertAgent(ctx, receiver); err != nil {
		t.Fatalf("InsertAgent(receiver) error = %v", err)
	}

	message := &db.Message{
		FromAgent: sender.ID,
		ToAgent:   receiver.ID,
		Type:      "notification",
		Payload:   `{"hello":"world"}`,
	}
	if err := database.InsertMessage(ctx, message); err != nil {
		t.Fatalf("InsertMessage() error = %v", err)
	}

	received := make(chan db.Message, 1)
	poller := NewPoller(database, receiver.ID, func(data []byte) {
		var msg db.Message
		if err := json.Unmarshal(data, &msg); err == nil {
			received <- msg
		}
	})
	poller.interval = 10 * time.Millisecond

	pollCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	poller.Start(pollCtx)

	select {
	case got := <-received:
		if got.ID != message.ID {
			t.Fatalf("polled message ID = %q, want %q", got.ID, message.ID)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for poller delivery")
	}

	stored, err := database.FindMessageByID(ctx, message.ID)
	if err != nil {
		t.Fatalf("FindMessageByID() error = %v", err)
	}
	if stored.DeliveredAt == "" {
		t.Fatal("DeliveredAt was not set by poller")
	}
}
