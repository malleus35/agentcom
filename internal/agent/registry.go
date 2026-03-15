package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	gonanoid "github.com/matoous/go-nanoid/v2"

	"github.com/malleus35/agentcom/internal/config"
	"github.com/malleus35/agentcom/internal/db"
)

const heartbeatStaleThreshold = 30 * time.Second

var (
	// ErrAgentNotFound indicates the target agent cannot be found.
	ErrAgentNotFound = errors.New("agent not found")
)

// Registry manages agent lifecycle.
type Registry struct {
	db  *db.DB
	cfg *config.Config
}

// NewRegistry creates a new registry with database and path configuration.
func NewRegistry(database *db.DB, cfg *config.Config) *Registry {
	return &Registry{
		db:  database,
		cfg: cfg,
	}
}

// Register registers a new agent and persists it in SQLite.
func (r *Registry) Register(ctx context.Context, name, agentType string, capabilities []string, workdir string, project string) (*db.Agent, error) {
	rawID, err := gonanoid.New()
	if err != nil {
		return nil, fmt.Errorf("agent.Register: %w", err)
	}

	capabilitiesJSON, err := json.Marshal(capabilities)
	if err != nil {
		return nil, fmt.Errorf("agent.Register: %w", err)
	}

	now := time.Now().UTC()
	agent := &db.Agent{
		ID:            "agt_" + rawID,
		Name:          name,
		Type:          agentType,
		PID:           os.Getpid(),
		SocketPath:    filepath.Join(r.cfg.SocketsPath, "agt_"+rawID+".sock"),
		Capabilities:  string(capabilitiesJSON),
		WorkDir:       workdir,
		Project:       project,
		Status:        "alive",
		RegisteredAt:  now,
		LastHeartbeat: now,
	}

	if err := r.db.InsertAgent(ctx, agent); err != nil {
		return nil, fmt.Errorf("agent.Register: %w", err)
	}

	slog.Debug("agent registered", "agent_id", agent.ID, "name", agent.Name)

	return agent, nil
}

// Deregister removes an agent by name or ID.
func (r *Registry) Deregister(ctx context.Context, nameOrID string, project string) error {
	agent, err := r.db.FindAgentByNameAndProject(ctx, nameOrID, project)
	if err != nil {
		agent, err = r.db.FindAgentByID(ctx, nameOrID)
		if err != nil {
			return fmt.Errorf("agent.Deregister: %w", errors.Join(ErrAgentNotFound, err))
		}
	}

	if err := r.db.DeleteAgent(ctx, agent.ID); err != nil {
		return fmt.Errorf("agent.Deregister: %w", err)
	}

	if err := os.Remove(agent.SocketPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("agent.Deregister: %w", err)
	}

	slog.Debug("agent deregistered", "agent_id", agent.ID, "name", agent.Name)

	return nil
}

// FindByName finds an agent by unique name.
func (r *Registry) FindByName(ctx context.Context, name string, project string) (*db.Agent, error) {
	agent, err := r.db.FindAgentByNameAndProject(ctx, name, project)
	if err != nil {
		return nil, fmt.Errorf("agent.FindByName: %w", err)
	}
	return agent, nil
}

// FindByID finds an agent by unique ID.
func (r *Registry) FindByID(ctx context.Context, id string) (*db.Agent, error) {
	agent, err := r.db.FindAgentByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("agent.FindByID: %w", err)
	}
	return agent, nil
}

// ListAll lists all registered agents.
func (r *Registry) ListAll(ctx context.Context, project string) ([]*db.Agent, error) {
	agents, err := r.db.ListAgentsByProject(ctx, project)
	if err != nil {
		return nil, fmt.Errorf("agent.ListAll: %w", err)
	}
	return agents, nil
}

// ListAlive lists agents currently marked as alive.
func (r *Registry) ListAlive(ctx context.Context, project string) ([]*db.Agent, error) {
	agents, err := r.db.ListAliveAgentsByProject(ctx, project)
	if err != nil {
		return nil, fmt.Errorf("agent.ListAlive: %w", err)
	}
	return agents, nil
}

// MarkInactive marks stale and dead-process agents as dead.
func (r *Registry) MarkInactive(ctx context.Context) error {
	agents, err := r.db.ListAliveAgents(ctx)
	if err != nil {
		return fmt.Errorf("agent.MarkInactive: %w", err)
	}

	now := time.Now().UTC()
	for _, a := range agents {
		if now.Sub(a.LastHeartbeat) <= heartbeatStaleThreshold {
			continue
		}
		if processAlive(a.PID) {
			continue
		}

		a.Status = "dead"
		if err := r.db.UpdateAgent(ctx, a); err != nil {
			return fmt.Errorf("agent.MarkInactive: %w", err)
		}

		slog.Debug("agent marked dead", "agent_id", a.ID, "name", a.Name, "pid", a.PID)
	}

	return nil
}
