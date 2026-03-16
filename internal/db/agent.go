package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	gonanoid "github.com/matoous/go-nanoid/v2"
	"github.com/mattn/go-sqlite3"
)

var ErrAgentNotFound = errors.New("agent not found")
var ErrDuplicateName = errors.New("agent name already exists in this project")

// Agent represents a registered agent record.
type Agent struct {
	ID            string
	Name          string
	Type          string
	PID           int
	SocketPath    string
	Capabilities  string
	WorkDir       string
	Project       string
	Status        string
	RegisteredAt  time.Time
	LastHeartbeat time.Time
}

// InsertAgent inserts a new agent row and generates an agent ID when needed.
func (d *DB) InsertAgent(ctx context.Context, agent *Agent) error {
	if agent.ID == "" {
		id, err := gonanoid.New()
		if err != nil {
			return fmt.Errorf("db.InsertAgent: generate id: %w", err)
		}
		agent.ID = "agt_" + id
	}
	if err := d.EnsureProject(ctx, agent.Project); err != nil {
		return fmt.Errorf("db.InsertAgent: ensure project: %w", err)
	}

	stmt, err := d.PrepareContext(ctx, `
		INSERT INTO agents (
			id, name, type, pid, socket_path, capabilities, workdir, project, status
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("db.InsertAgent: prepare: %w", err)
	}
	defer stmt.Close()

	if _, err := stmt.ExecContext(ctx,
		agent.ID,
		agent.Name,
		agent.Type,
		agent.PID,
		nullableString(agent.SocketPath),
		agent.Capabilities,
		nullableString(agent.WorkDir),
		agent.Project,
		agent.Status,
	); err != nil {
		if isUniqueNameViolation(err) {
			return ErrDuplicateName
		}
		return fmt.Errorf("db.InsertAgent: exec: %w", err)
	}

	return nil
}

// UpdateAgent updates mutable fields for an existing agent.
func (d *DB) UpdateAgent(ctx context.Context, agent *Agent) error {
	if err := d.EnsureProject(ctx, agent.Project); err != nil {
		return fmt.Errorf("db.UpdateAgent: ensure project: %w", err)
	}

	stmt, err := d.PrepareContext(ctx, `
		UPDATE agents
		SET name = ?, type = ?, pid = ?, socket_path = ?, capabilities = ?, workdir = ?, project = ?, status = ?
		WHERE id = ?
	`)
	if err != nil {
		return fmt.Errorf("db.UpdateAgent: prepare: %w", err)
	}
	defer stmt.Close()

	res, err := stmt.ExecContext(ctx,
		agent.Name,
		agent.Type,
		agent.PID,
		nullableString(agent.SocketPath),
		agent.Capabilities,
		nullableString(agent.WorkDir),
		agent.Project,
		agent.Status,
		agent.ID,
	)
	if err != nil {
		if isUniqueNameViolation(err) {
			return ErrDuplicateName
		}
		return fmt.Errorf("db.UpdateAgent: exec: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("db.UpdateAgent: rows affected: %w", err)
	}
	if rows == 0 {
		return ErrAgentNotFound
	}

	return nil
}

// DeleteAgent deletes an agent by ID.
func (d *DB) DeleteAgent(ctx context.Context, id string) error {
	stmt, err := d.PrepareContext(ctx, `DELETE FROM agents WHERE id = ?`)
	if err != nil {
		return fmt.Errorf("db.DeleteAgent: prepare: %w", err)
	}
	defer stmt.Close()

	res, err := stmt.ExecContext(ctx, id)
	if err != nil {
		return fmt.Errorf("db.DeleteAgent: exec: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("db.DeleteAgent: rows affected: %w", err)
	}
	if rows == 0 {
		return ErrAgentNotFound
	}

	return nil
}

// FindAgentByName finds a single agent by its unique name.
func (d *DB) FindAgentByName(ctx context.Context, name string) (*Agent, error) {
	stmt, err := d.PrepareContext(ctx, `
		SELECT
			id, name, type, pid, socket_path, capabilities, workdir, project, status, registered_at, last_heartbeat
		FROM agents
		WHERE name = ?
		ORDER BY registered_at ASC
		LIMIT 1
	`)
	if err != nil {
		return nil, fmt.Errorf("db.FindAgentByName: prepare: %w", err)
	}
	defer stmt.Close()

	agent, err := scanAgent(stmt.QueryRowContext(ctx, name))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrAgentNotFound
		}
		return nil, fmt.Errorf("db.FindAgentByName: %w", err)
	}

	return agent, nil
}

// FindAgentByNameAndProject finds a single agent by name within a project.
func (d *DB) FindAgentByNameAndProject(ctx context.Context, name string, project string) (*Agent, error) {
	if project == "" {
		return d.FindAgentByName(ctx, name)
	}

	stmt, err := d.PrepareContext(ctx, `
		SELECT
			id, name, type, pid, socket_path, capabilities, workdir, project, status, registered_at, last_heartbeat
		FROM agents
		WHERE name = ? AND project = ?
		ORDER BY registered_at ASC
		LIMIT 1
	`)
	if err != nil {
		return nil, fmt.Errorf("db.FindAgentByNameAndProject: prepare: %w", err)
	}
	defer stmt.Close()

	agent, err := scanAgent(stmt.QueryRowContext(ctx, name, project))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrAgentNotFound
		}
		return nil, fmt.Errorf("db.FindAgentByNameAndProject: %w", err)
	}

	return agent, nil
}

// FindAgentByID finds a single agent by ID.
func (d *DB) FindAgentByID(ctx context.Context, id string) (*Agent, error) {
	stmt, err := d.PrepareContext(ctx, `
		SELECT
			id, name, type, pid, socket_path, capabilities, workdir, project, status, registered_at, last_heartbeat
		FROM agents
		WHERE id = ?
	`)
	if err != nil {
		return nil, fmt.Errorf("db.FindAgentByID: prepare: %w", err)
	}
	defer stmt.Close()

	agent, err := scanAgent(stmt.QueryRowContext(ctx, id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrAgentNotFound
		}
		return nil, fmt.Errorf("db.FindAgentByID: %w", err)
	}

	return agent, nil
}

// ListAllAgents lists all agents in registration order.
func (d *DB) ListAllAgents(ctx context.Context) ([]*Agent, error) {
	stmt, err := d.PrepareContext(ctx, `
		SELECT
			id, name, type, pid, socket_path, capabilities, workdir, project, status, registered_at, last_heartbeat
		FROM agents
		ORDER BY registered_at ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("db.ListAllAgents: prepare: %w", err)
	}
	defer stmt.Close()

	rows, err := stmt.QueryContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("db.ListAllAgents: query: %w", err)
	}
	defer rows.Close()

	agents := make([]*Agent, 0)
	for rows.Next() {
		agent, err := scanAgent(rows)
		if err != nil {
			return nil, fmt.Errorf("db.ListAllAgents: scan: %w", err)
		}
		agents = append(agents, agent)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db.ListAllAgents: rows: %w", err)
	}

	return agents, nil
}

// ListAgentsByProject lists all agents for a project, or all agents in legacy mode.
func (d *DB) ListAgentsByProject(ctx context.Context, project string) ([]*Agent, error) {
	if project == "" {
		return d.ListAllAgents(ctx)
	}

	stmt, err := d.PrepareContext(ctx, `
		SELECT
			id, name, type, pid, socket_path, capabilities, workdir, project, status, registered_at, last_heartbeat
		FROM agents
		WHERE project = ?
		ORDER BY registered_at ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("db.ListAgentsByProject: prepare: %w", err)
	}
	defer stmt.Close()

	rows, err := stmt.QueryContext(ctx, project)
	if err != nil {
		return nil, fmt.Errorf("db.ListAgentsByProject: query: %w", err)
	}
	defer rows.Close()

	agents := make([]*Agent, 0)
	for rows.Next() {
		agent, err := scanAgent(rows)
		if err != nil {
			return nil, fmt.Errorf("db.ListAgentsByProject: scan: %w", err)
		}
		agents = append(agents, agent)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db.ListAgentsByProject: rows: %w", err)
	}

	return agents, nil
}

// ListAliveAgents lists all agents with alive status.
func (d *DB) ListAliveAgents(ctx context.Context) ([]*Agent, error) {
	stmt, err := d.PrepareContext(ctx, `
		SELECT
			id, name, type, pid, socket_path, capabilities, workdir, project, status, registered_at, last_heartbeat
		FROM agents
		WHERE status = 'alive'
		ORDER BY last_heartbeat DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("db.ListAliveAgents: prepare: %w", err)
	}
	defer stmt.Close()

	rows, err := stmt.QueryContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("db.ListAliveAgents: query: %w", err)
	}
	defer rows.Close()

	agents := make([]*Agent, 0)
	for rows.Next() {
		agent, err := scanAgent(rows)
		if err != nil {
			return nil, fmt.Errorf("db.ListAliveAgents: scan: %w", err)
		}
		agents = append(agents, agent)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db.ListAliveAgents: rows: %w", err)
	}

	return agents, nil
}

// ListAliveAgentsByProject lists alive agents for a project, or all alive agents in legacy mode.
func (d *DB) ListAliveAgentsByProject(ctx context.Context, project string) ([]*Agent, error) {
	if project == "" {
		return d.ListAliveAgents(ctx)
	}

	stmt, err := d.PrepareContext(ctx, `
		SELECT
			id, name, type, pid, socket_path, capabilities, workdir, project, status, registered_at, last_heartbeat
		FROM agents
		WHERE status = 'alive' AND project = ?
		ORDER BY last_heartbeat DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("db.ListAliveAgentsByProject: prepare: %w", err)
	}
	defer stmt.Close()

	rows, err := stmt.QueryContext(ctx, project)
	if err != nil {
		return nil, fmt.Errorf("db.ListAliveAgentsByProject: query: %w", err)
	}
	defer rows.Close()

	agents := make([]*Agent, 0)
	for rows.Next() {
		agent, err := scanAgent(rows)
		if err != nil {
			return nil, fmt.Errorf("db.ListAliveAgentsByProject: scan: %w", err)
		}
		agents = append(agents, agent)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db.ListAliveAgentsByProject: rows: %w", err)
	}

	return agents, nil
}

func (d *DB) ListProjects(ctx context.Context) ([]string, error) {
	stmt, err := d.PrepareContext(ctx, `
		SELECT name
		FROM projects
		ORDER BY name ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("db.ListProjects: prepare: %w", err)
	}
	defer stmt.Close()

	rows, err := stmt.QueryContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("db.ListProjects: query: %w", err)
	}
	defer rows.Close()

	projects := make([]string, 0)
	for rows.Next() {
		var project string
		if err := rows.Scan(&project); err != nil {
			return nil, fmt.Errorf("db.ListProjects: scan: %w", err)
		}
		projects = append(projects, project)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db.ListProjects: rows: %w", err)
	}

	return projects, nil
}

func (d *DB) ProjectsTableExists(ctx context.Context) (bool, error) {
	var count int
	if err := d.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, "projects").Scan(&count); err != nil {
		return false, fmt.Errorf("db.ProjectsTableExists: query: %w", err)
	}

	return count > 0, nil
}

func (d *DB) EnsureProject(ctx context.Context, project string) error {
	if project == "" {
		return nil
	}

	stmt, err := d.PrepareContext(ctx, `
		INSERT OR IGNORE INTO projects (name) VALUES (?)
	`)
	if err != nil {
		return fmt.Errorf("db.EnsureProject: prepare: %w", err)
	}
	defer stmt.Close()

	if _, err := stmt.ExecContext(ctx, project); err != nil {
		return fmt.Errorf("db.EnsureProject: exec: %w", err)
	}

	return nil
}

// UpdateHeartbeat updates an agent heartbeat and marks it alive.
func (d *DB) UpdateHeartbeat(ctx context.Context, id string) error {
	stmt, err := d.PrepareContext(ctx, `
		UPDATE agents
		SET last_heartbeat = datetime('now'), status = 'alive'
		WHERE id = ?
	`)
	if err != nil {
		return fmt.Errorf("db.UpdateHeartbeat: prepare: %w", err)
	}
	defer stmt.Close()

	res, err := stmt.ExecContext(ctx, id)
	if err != nil {
		return fmt.Errorf("db.UpdateHeartbeat: exec: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("db.UpdateHeartbeat: rows affected: %w", err)
	}
	if rows == 0 {
		return ErrAgentNotFound
	}

	return nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanAgent(scanner rowScanner) (*Agent, error) {
	var socketPath sql.NullString
	var workDir sql.NullString
	var project sql.NullString
	var pid sql.NullInt64
	var registeredAt sql.NullString
	var lastHeartbeat sql.NullString

	agent := &Agent{}
	if err := scanner.Scan(
		&agent.ID,
		&agent.Name,
		&agent.Type,
		&pid,
		&socketPath,
		&agent.Capabilities,
		&workDir,
		&project,
		&agent.Status,
		&registeredAt,
		&lastHeartbeat,
	); err != nil {
		return nil, err
	}

	if pid.Valid {
		agent.PID = int(pid.Int64)
	}
	if socketPath.Valid {
		agent.SocketPath = socketPath.String
	}
	if workDir.Valid {
		agent.WorkDir = workDir.String
	}
	if project.Valid {
		agent.Project = project.String
	}

	if registeredAt.Valid {
		t, err := time.Parse(time.DateTime, registeredAt.String)
		if err != nil {
			return nil, fmt.Errorf("db.scanAgent: parse registered_at: %w", err)
		}
		agent.RegisteredAt = t
	}

	if lastHeartbeat.Valid {
		t, err := time.Parse(time.DateTime, lastHeartbeat.String)
		if err != nil {
			return nil, fmt.Errorf("db.scanAgent: parse last_heartbeat: %w", err)
		}
		agent.LastHeartbeat = t
	}

	return agent, nil
}

func nullableString(v string) any {
	if v == "" {
		return nil
	}
	return v
}

func isUniqueNameViolation(err error) bool {
	var sqliteErr sqlite3.Error
	if !errors.As(err, &sqliteErr) {
		return false
	}

	return sqliteErr.ExtendedCode == sqlite3.ErrConstraintUnique
}
