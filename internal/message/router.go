package message

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/malleus35/agentcom/internal/db"
)

// AgentFinder locates agents for message routing.
type AgentFinder interface {
	FindByName(ctx context.Context, name string, project string) (*db.Agent, error)
	FindByID(ctx context.Context, id string) (*db.Agent, error)
	ListAlive(ctx context.Context, project string) ([]*db.Agent, error)
}

// Transport sends raw message bytes to an agent's socket.
type Transport interface {
	Send(ctx context.Context, socketPath string, data []byte) error
}

// Router routes direct and broadcast messages.
type Router struct {
	db        *db.DB
	finder    AgentFinder
	transport Transport
	project   string
}

// NewRouter creates a message router.
func NewRouter(database *db.DB, finder AgentFinder, transport Transport, project string) *Router {
	return &Router{
		db:        database,
		finder:    finder,
		transport: transport,
		project:   project,
	}
}

// Send routes a message to a single target agent and persists it.
func (r *Router) Send(
	ctx context.Context,
	from string,
	toNameOrID string,
	msgType string,
	topic string,
	payload json.RawMessage,
) (*Envelope, error) {
	target, err := r.finder.FindByName(ctx, toNameOrID, r.project)
	if err != nil {
		target, err = r.finder.FindByID(ctx, toNameOrID)
		if err != nil {
			return nil, fmt.Errorf("message.Router.Send: resolve target: %w", err)
		}
	}

	env := NewEnvelope(from, target.ID, msgType, topic, payload)
	data, err := json.Marshal(env)
	if err != nil {
		return nil, fmt.Errorf("message.Router.Send: marshal envelope: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	stored := &db.Message{
		ID:            env.ID,
		FromAgent:     env.From,
		ToAgent:       env.To,
		Type:          env.Type,
		Topic:         env.Topic,
		Payload:       string(env.Payload),
		CorrelationID: env.CorrelationID,
		CreatedAt:     now,
	}

	if target.SocketPath == "" {
		slog.Debug("message routed to sqlite inbox (missing socket)", "to", target.ID, "topic", topic)
		if err := r.db.InsertMessage(ctx, stored); err != nil {
			return nil, fmt.Errorf("message.Router.Send: insert fallback message: %w", err)
		}
		return env, nil
	}

	if err := r.transport.Send(ctx, target.SocketPath, data); err != nil {
		slog.Debug("transport send failed; routing to sqlite inbox", "to", target.ID, "socket", target.SocketPath, "error", err)
		if insertErr := r.db.InsertMessage(ctx, stored); insertErr != nil {
			return nil, fmt.Errorf("message.Router.Send: insert fallback message: %w", insertErr)
		}
		return env, nil
	}

	stored.DeliveredAt = now
	if err := r.db.InsertMessage(ctx, stored); err != nil {
		return nil, fmt.Errorf("message.Router.Send: insert audit message: %w", err)
	}

	return env, nil
}

// Broadcast sends a message to all alive agents except the sender.
func (r *Router) Broadcast(
	ctx context.Context,
	from string,
	topic string,
	payload json.RawMessage,
) ([]*Envelope, error) {
	agents, err := r.finder.ListAlive(ctx, r.project)
	if err != nil {
		return nil, fmt.Errorf("message.Router.Broadcast: list alive agents: %w", err)
	}

	envelopes := make([]*Envelope, 0, len(agents))
	for _, agent := range agents {
		if agent.ID == from || agent.Name == from {
			continue
		}

		env, sendErr := r.Send(ctx, from, agent.ID, "broadcast", topic, payload)
		if sendErr != nil {
			slog.Debug("broadcast send failed", "from", from, "to", agent.ID, "topic", topic, "error", sendErr)
			continue
		}

		envelopes = append(envelopes, env)
	}

	return envelopes, nil
}
