package transport

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/malleus35/agentcom/internal/db"
)

// Poller periodically checks the SQLite inbox for undelivered messages.
type Poller struct {
	db       *db.DB
	agentID  string
	interval time.Duration
	handler  MessageHandler
}

// NewPoller creates a polling-based fallback inbox consumer.
func NewPoller(database *db.DB, agentID string, handler MessageHandler) *Poller {
	return &Poller{
		db:       database,
		agentID:  agentID,
		interval: 5 * time.Second,
		handler:  handler,
	}
}

// Start begins polling unread messages until context cancellation.
func (p *Poller) Start(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(p.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				messages, err := p.db.ListUnreadMessages(ctx, p.agentID)
				if err != nil {
					slog.Debug("fallback poll list unread failed", "agent_id", p.agentID, "error", err)
					continue
				}

				for _, msg := range messages {
					data, marshalErr := json.Marshal(msg)
					if marshalErr != nil {
						slog.Debug("fallback poll marshal message failed", "message_id", msg.ID, "error", marshalErr)
						continue
					}

					p.handler(data)

					if markErr := p.db.MarkDelivered(ctx, msg.ID); markErr != nil {
						slog.Debug("fallback poll mark delivered failed", "message_id", msg.ID, "error", markErr)
					}
				}
			}
		}
	}()
}
