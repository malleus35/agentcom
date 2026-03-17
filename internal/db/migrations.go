package db

import (
	"context"
	"database/sql"
	"fmt"
)

var migrations = []string{
	`CREATE TABLE IF NOT EXISTS agents (
		id             TEXT PRIMARY KEY,
		name           TEXT NOT NULL UNIQUE,
		type           TEXT NOT NULL,
		pid            INTEGER,
		socket_path    TEXT,
		capabilities   TEXT DEFAULT '[]',
		workdir        TEXT,
		status         TEXT NOT NULL DEFAULT 'alive',
		registered_at  TEXT NOT NULL DEFAULT (datetime('now')),
		last_heartbeat TEXT NOT NULL DEFAULT (datetime('now'))
	)`,
	`CREATE TABLE IF NOT EXISTS messages (
		id             TEXT PRIMARY KEY,
		from_agent     TEXT NOT NULL,
		to_agent       TEXT,
		type           TEXT NOT NULL DEFAULT 'notification',
		topic          TEXT,
		payload        TEXT DEFAULT '{}',
		correlation_id TEXT,
		created_at     TEXT NOT NULL DEFAULT (datetime('now')),
		delivered_at   TEXT,
		read_at        TEXT
	)`,
	`CREATE TABLE IF NOT EXISTS tasks (
		id          TEXT PRIMARY KEY,
		title       TEXT NOT NULL,
		description TEXT,
		status      TEXT NOT NULL DEFAULT 'pending',
		priority    TEXT NOT NULL DEFAULT 'medium',
		assigned_to TEXT,
		created_by  TEXT,
		blocked_by  TEXT DEFAULT '[]',
		result      TEXT,
		created_at  TEXT NOT NULL DEFAULT (datetime('now')),
		updated_at  TEXT NOT NULL DEFAULT (datetime('now'))
	)`,
	`CREATE INDEX IF NOT EXISTS idx_messages_to_agent ON messages(to_agent, delivered_at)`,
	`CREATE INDEX IF NOT EXISTS idx_messages_correlation ON messages(correlation_id)`,
	`CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status)`,
	`CREATE INDEX IF NOT EXISTS idx_tasks_assignee ON tasks(assigned_to)`,
	`CREATE INDEX IF NOT EXISTS idx_agents_status ON agents(status)`,
	`CREATE TABLE agents_new (
		id             TEXT PRIMARY KEY,
		name           TEXT NOT NULL,
		type           TEXT NOT NULL,
		pid            INTEGER,
		socket_path    TEXT,
		capabilities   TEXT DEFAULT '[]',
		workdir        TEXT,
		project        TEXT NOT NULL DEFAULT '',
		status         TEXT NOT NULL DEFAULT 'alive',
		registered_at  TEXT NOT NULL DEFAULT (datetime('now')),
		last_heartbeat TEXT NOT NULL DEFAULT (datetime('now')),
		UNIQUE(name, project)
	);
	INSERT INTO agents_new (
		id, name, type, pid, socket_path, capabilities, workdir, project, status, registered_at, last_heartbeat
	)
	SELECT
		id, name, type, pid, socket_path, capabilities, workdir, '', status, registered_at, last_heartbeat
	FROM agents;
	DROP TABLE agents;
	ALTER TABLE agents_new RENAME TO agents;
	CREATE INDEX idx_agents_status ON agents(status);
	CREATE INDEX idx_agents_project ON agents(project);`,
	`CREATE TABLE IF NOT EXISTS projects (
		name       TEXT PRIMARY KEY,
		created_at TEXT NOT NULL DEFAULT (datetime('now'))
	);
	INSERT OR IGNORE INTO projects (name)
	SELECT DISTINCT project
	FROM agents
	WHERE project IS NOT NULL AND project <> '';`,
	`ALTER TABLE tasks ADD COLUMN reviewer TEXT DEFAULT ''`,
}

// Migrate applies all pending schema migrations.
func (d *DB) Migrate(ctx context.Context) error {
	tx, err := d.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("db.Migrate: begin: %w", err)
	}
	defer tx.Rollback()

	version, err := schemaVersion(ctx, tx)
	if err != nil {
		return fmt.Errorf("db.Migrate: schema version: %w", err)
	}

	for i, m := range migrations {
		if i < version {
			continue
		}
		if _, err := tx.ExecContext(ctx, m); err != nil {
			return fmt.Errorf("db.Migrate: migration %d: %w", i, err)
		}
		if err := setSchemaVersion(ctx, tx, i+1); err != nil {
			return fmt.Errorf("db.Migrate: set schema version %d: %w", i+1, err)
		}
	}

	return tx.Commit()
}

func schemaVersion(ctx context.Context, tx *sql.Tx) (int, error) {
	var version int
	if err := tx.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&version); err != nil {
		return 0, fmt.Errorf("db.schemaVersion: %w", err)
	}
	return version, nil
}

func setSchemaVersion(ctx context.Context, tx *sql.Tx, version int) error {
	if _, err := tx.ExecContext(ctx, fmt.Sprintf(`PRAGMA user_version = %d`, version)); err != nil {
		return fmt.Errorf("db.setSchemaVersion: %w", err)
	}
	return nil
}
