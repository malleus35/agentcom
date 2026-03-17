package message

import (
	"context"
	"encoding/json"
	"errors"
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

var (
	ErrSendRateLimited      = errors.New("send rate limited")
	ErrBroadcastThrottled   = errors.New("broadcast throttled")
	inboxLimitPerAgent      = 1000
	sendRateLimitPerWindow  = 100
	sendRateLimitWindow     = time.Second
	broadcastThrottleWindow = time.Second
)

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
	if err := r.enforceMessagePolicies(ctx, from, target.ID, msgType, topic); err != nil {
		return nil, fmt.Errorf("message.Router.Send: %w", err)
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
		slog.Info("message routed to sqlite inbox", "to", target.ID, "reason", "missing_socket", "topic", topic)
		if err := r.db.InsertMessage(ctx, stored); err != nil {
			return nil, fmt.Errorf("message.Router.Send: insert fallback message: %w", err)
		}
		return env, nil
	}

	if err := r.transport.Send(ctx, target.SocketPath, data); err != nil {
		slog.Warn("transport send failed; routing to sqlite inbox", "to", target.ID, "socket", target.SocketPath, "error", err)
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
	if topic != "" && broadcastThrottleWindow > 0 {
		exists, err := r.hasRecentBroadcast(ctx, from, topic, time.Now().Add(-broadcastThrottleWindow))
		if err != nil {
			return nil, fmt.Errorf("message.Router.Broadcast: %w", err)
		}
		if exists {
			return nil, fmt.Errorf("message.Router.Broadcast: %w", ErrBroadcastThrottled)
		}
	}

	agents, err := r.finder.ListAlive(ctx, r.project)
	if err != nil {
		return nil, fmt.Errorf("message.Router.Broadcast: list alive agents: %w", err)
	}

	envelopes := make([]*Envelope, 0, len(agents))
	for _, agent := range agents {
		if agent.ID == from || agent.Name == from {
			continue
		}
		if agent.Type == "human" {
			continue
		}

		env, sendErr := r.Send(ctx, from, agent.ID, "broadcast", topic, payload)
		if sendErr != nil {
			slog.Warn("broadcast send failed", "from", from, "to", agent.ID, "topic", topic, "error", sendErr)
			continue
		}

		envelopes = append(envelopes, env)
	}

	return envelopes, nil
}

func (r *Router) enforceMessagePolicies(ctx context.Context, from, targetID, msgType, topic string) error {
	if sendRateLimitPerWindow > 0 {
		count, err := r.countRecentMessagesFromAgent(ctx, from, time.Now().Add(-sendRateLimitWindow))
		if err != nil {
			return err
		}
		if count >= sendRateLimitPerWindow {
			return ErrSendRateLimited
		}
	}
	if inboxLimitPerAgent > 0 {
		if err := r.trimInboxForAgent(ctx, targetID, inboxLimitPerAgent-1); err != nil {
			return err
		}
	}
	return nil
}

func (r *Router) countRecentMessagesFromAgent(ctx context.Context, agentID string, since time.Time) (int, error) {
	var count int
	if err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM messages
		WHERE from_agent = ? AND datetime(created_at) >= datetime(?)
	`, agentID, since.Format(time.RFC3339)).Scan(&count); err != nil {
		return 0, fmt.Errorf("message.Router.countRecentMessagesFromAgent: %w", err)
	}
	return count, nil
}

func (r *Router) hasRecentBroadcast(ctx context.Context, from, topic string, since time.Time) (bool, error) {
	var count int
	if err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM messages
		WHERE from_agent = ? AND type = 'broadcast' AND topic = ? AND datetime(created_at) >= datetime(?)
	`, from, topic, since.Format(time.RFC3339)).Scan(&count); err != nil {
		return false, fmt.Errorf("message.Router.hasRecentBroadcast: %w", err)
	}
	return count > 0, nil
}

func (r *Router) trimInboxForAgent(ctx context.Context, agentID string, keep int) error {
	for {
		var count int
		if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM messages WHERE to_agent = ?`, agentID).Scan(&count); err != nil {
			return fmt.Errorf("message.Router.trimInboxForAgent: count: %w", err)
		}
		if count <= keep {
			return nil
		}
		if _, err := r.db.ExecContext(ctx, `
			DELETE FROM messages
			WHERE id = (
				SELECT id FROM messages WHERE to_agent = ? ORDER BY datetime(created_at) ASC, id ASC LIMIT 1
			)
		`, agentID); err != nil {
			return fmt.Errorf("message.Router.trimInboxForAgent: delete oldest: %w", err)
		}
	}
}
