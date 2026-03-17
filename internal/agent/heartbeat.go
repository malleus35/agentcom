package agent

import (
	"context"
	"log/slog"
	"time"

	"github.com/malleus35/agentcom/internal/config"
	"github.com/malleus35/agentcom/internal/db"
)

var defaultHeartbeatInterval = 10 * time.Second

// Heartbeat runs a background goroutine that updates the agent heartbeat.
type Heartbeat struct {
	db       *db.DB
	agentID  string
	interval time.Duration
}

// NewHeartbeat creates a new heartbeat runner for one agent.
func NewHeartbeat(database *db.DB, agentID string) *Heartbeat {
	return &Heartbeat{
		db:       database,
		agentID:  agentID,
		interval: defaultHeartbeatInterval,
	}
}

// Start starts the heartbeat ticker loop and stops when context is done.
func (h *Heartbeat) Start(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(h.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := h.db.UpdateHeartbeat(ctx, h.agentID); err != nil {
					slog.Error("heartbeat update failed", "agent_id", h.agentID, "error", err)
				}
			}
		}
	}()
}

func ApplyHeartbeatRuntimeConfig(runtime config.RuntimeConfig) {
	defaultHeartbeatInterval = runtime.HeartbeatInterval
}
