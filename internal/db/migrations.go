package db

import (
	"context"
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
}

// Migrate applies all pending schema migrations.
func (d *DB) Migrate(ctx context.Context) error {
	tx, err := d.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("db.Migrate: begin: %w", err)
	}
	defer tx.Rollback()

	for i, m := range migrations {
		if _, err := tx.ExecContext(ctx, m); err != nil {
			return fmt.Errorf("db.Migrate: migration %d: %w", i, err)
		}
	}

	return tx.Commit()
}
