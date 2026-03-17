package message

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/malleus35/agentcom/internal/db"
)

type stubFinder struct {
	byName map[string]*db.Agent
	byID   map[string]*db.Agent
	alive  []*db.Agent
}

func (s *stubFinder) FindByName(ctx context.Context, name string, project string) (*db.Agent, error) {
	if agent, ok := s.byName[name]; ok {
		if project != "" && agent.Project != project {
			return nil, db.ErrAgentNotFound
		}
		return agent, nil
	}
	return nil, db.ErrAgentNotFound
}

func (s *stubFinder) FindByID(ctx context.Context, id string) (*db.Agent, error) {
	if agent, ok := s.byID[id]; ok {
		return agent, nil
	}
	return nil, db.ErrAgentNotFound
}

func (s *stubFinder) ListAlive(ctx context.Context, project string) ([]*db.Agent, error) {
	if project == "" {
		return s.alive, nil
	}
	filtered := make([]*db.Agent, 0, len(s.alive))
	for _, agent := range s.alive {
		if agent.Project == project {
			filtered = append(filtered, agent)
		}
	}
	return filtered, nil
}

type stubTransport struct {
	err      error
	lastPath string
	lastData []byte
}

func (s *stubTransport) Send(ctx context.Context, socketPath string, data []byte) error {
	s.lastPath = socketPath
	s.lastData = append([]byte(nil), data...)
	return s.err
}

func setupMessageTestDB(t *testing.T) *db.DB {
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

func TestRouterSendFallsBackToSQLiteWhenSocketMissing(t *testing.T) {
	database := setupMessageTestDB(t)
	target := &db.Agent{ID: "agt_target", Name: "beta", Project: "project-a", Status: "alive"}
	finder := &stubFinder{
		byName: map[string]*db.Agent{"beta": target},
		byID:   map[string]*db.Agent{target.ID: target},
	}
	transport := &stubTransport{}
	router := NewRouter(database, finder, transport, "project-a")

	env, err := router.Send(context.Background(), "agt_sender", "beta", "notification", "sync", json.RawMessage(`{"ok":true}`))
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if env.ID == "" {
		t.Fatal("env.ID is empty")
	}
	if transport.lastPath != "" {
		t.Fatalf("transport should not be used when socket missing, got %q", transport.lastPath)
	}

	messages, err := database.ListMessagesForAgent(context.Background(), target.ID)
	if err != nil {
		t.Fatalf("ListMessagesForAgent() error = %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("len(ListMessagesForAgent()) = %d, want 1", len(messages))
	}
	stored := messages[0]
	if stored.DeliveredAt != "" {
		t.Fatalf("DeliveredAt = %q, want empty fallback message", stored.DeliveredAt)
	}
}

func TestRouterSendMarksDeliveredOnTransportSuccess(t *testing.T) {
	database := setupMessageTestDB(t)
	target := &db.Agent{ID: "agt_target", Name: "beta", Project: "project-a", SocketPath: "/tmp/beta.sock", Status: "alive"}
	finder := &stubFinder{
		byName: map[string]*db.Agent{"beta": target},
		byID:   map[string]*db.Agent{target.ID: target},
	}
	transport := &stubTransport{}
	router := NewRouter(database, finder, transport, "project-a")

	_, err := router.Send(context.Background(), "agt_sender", "beta", "request", "review", json.RawMessage(`{"file":"README.md"}`))
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if transport.lastPath != target.SocketPath {
		t.Fatalf("transport.lastPath = %q, want %q", transport.lastPath, target.SocketPath)
	}

	messages, err := database.ListMessagesForAgent(context.Background(), target.ID)
	if err != nil {
		t.Fatalf("ListMessagesForAgent() error = %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("len(ListMessagesForAgent()) = %d, want 1", len(messages))
	}
	stored := messages[0]
	if stored.DeliveredAt == "" {
		t.Fatal("DeliveredAt is empty, want delivered audit message")
	}
}

func TestRouterBroadcastSkipsSenderAndContinuesOnFailure(t *testing.T) {
	database := setupMessageTestDB(t)
	alpha := &db.Agent{ID: "agt_alpha", Name: "alpha", Project: "project-a", Status: "alive"}
	beta := &db.Agent{ID: "agt_beta", Name: "beta", Project: "project-a", SocketPath: "/tmp/beta.sock", Status: "alive"}
	gamma := &db.Agent{ID: "agt_gamma", Name: "gamma", Project: "project-a", SocketPath: "/tmp/gamma.sock", Status: "alive"}
	user := &db.Agent{ID: "agt_user", Name: "user", Type: "human", Project: "project-a", Status: "alive"}

	finder := &stubFinder{alive: []*db.Agent{alpha, beta, gamma, user}, byID: map[string]*db.Agent{alpha.ID: alpha, beta.ID: beta, gamma.ID: gamma, user.ID: user}, byName: map[string]*db.Agent{alpha.Name: alpha, beta.Name: beta, gamma.Name: gamma, user.Name: user}}
	transport := &stubTransport{err: errors.New("send failed")}
	router := NewRouter(database, finder, transport, "project-a")

	envelopes, err := router.Broadcast(context.Background(), alpha.ID, "sync", json.RawMessage(`{"phase":9}`))
	if err != nil {
		t.Fatalf("Broadcast() error = %v", err)
	}
	if len(envelopes) != 2 {
		t.Fatalf("len(Broadcast()) = %d, want 2 fallback envelopes", len(envelopes))
	}

	for _, recipient := range []string{beta.ID, gamma.ID} {
		stored, err := database.ListMessagesForAgent(context.Background(), recipient)
		if err != nil {
			t.Fatalf("ListMessagesForAgent(%q) error = %v", recipient, err)
		}
		if len(stored) != 1 {
			t.Fatalf("len(ListMessagesForAgent(%q)) = %d, want 1", recipient, len(stored))
		}
		if stored[0].ToAgent == alpha.ID {
			t.Fatalf("broadcast stored sender as recipient: %+v", stored[0])
		}
	}

	userMessages, err := database.ListMessagesForAgent(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("ListMessagesForAgent(user) error = %v", err)
	}
	if len(userMessages) != 0 {
		t.Fatalf("len(ListMessagesForAgent(user)) = %d, want 0", len(userMessages))
	}

	if _, err := router.Send(context.Background(), alpha.ID, user.ID, "request", "approval", json.RawMessage(`{"text":"continue?"}`)); err != nil {
		t.Fatalf("Send(user) error = %v", err)
	}
	userMessages, err = database.ListMessagesForAgent(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("ListMessagesForAgent(user after direct send) error = %v", err)
	}
	if len(userMessages) != 1 {
		t.Fatalf("len(ListMessagesForAgent(user after direct send)) = %d, want 1", len(userMessages))
	}
}
