package db

import (
	"context"
	"fmt"
	"testing"
)

func TestMigrateFreshDatabase(t *testing.T) {
	database, err := OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory() error = %v", err)
	}
	t.Cleanup(func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	})

	ctx := context.Background()
	if err := database.Migrate(ctx); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	if got := currentSchemaVersion(t, database, ctx); got != len(migrations) {
		t.Fatalf("user_version = %d, want %d", got, len(migrations))
	}

	columns := agentColumns(t, database, ctx)
	project, ok := columns["project"]
	if !ok {
		t.Fatalf("agents columns missing project: %#v", columns)
	}
	if project.notNull != 1 {
		t.Fatalf("project notnull = %d, want 1", project.notNull)
	}
	if project.defaultValue != "''" {
		t.Fatalf("project default = %q, want %q", project.defaultValue, "''")
	}

	if !hasIndex(t, database, ctx, "agents", "idx_agents_project") {
		t.Fatal("idx_agents_project missing")
	}
	if !hasUniqueNameProjectIndex(t, database, ctx) {
		t.Fatal("unique (name, project) index missing")
	}
	if !hasTable(t, database, ctx, "projects") {
		t.Fatal("projects table missing")
	}

	taskColumns := tableColumns(t, database, ctx, "tasks")
	reviewer, ok := taskColumns["reviewer"]
	if !ok {
		t.Fatalf("tasks columns missing reviewer: %#v", taskColumns)
	}
	if reviewer.defaultValue != "''" {
		t.Fatalf("reviewer default = %q, want %q", reviewer.defaultValue, "''")
	}
}

func TestMigrateLegacyAgentsTable(t *testing.T) {
	database, err := OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory() error = %v", err)
	}
	t.Cleanup(func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	})

	ctx := context.Background()
	for _, stmt := range legacySchemaStatements() {
		if _, err := database.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("ExecContext(%q) error = %v", stmt, err)
		}
	}
	if _, err := database.ExecContext(ctx, `INSERT INTO agents (id, name, type, status) VALUES ('agt_legacy', 'alpha', 'worker', 'alive')`); err != nil {
		t.Fatalf("ExecContext(insert legacy agent) error = %v", err)
	}

	if err := database.Migrate(ctx); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	agent, err := database.FindAgentByID(ctx, "agt_legacy")
	if err != nil {
		t.Fatalf("FindAgentByID() error = %v", err)
	}
	if agent.Project != "" {
		t.Fatalf("Project = %q, want empty string", agent.Project)
	}
	if agent.Name != "alpha" {
		t.Fatalf("Name = %q, want alpha", agent.Name)
	}

	if got := currentSchemaVersion(t, database, ctx); got != len(migrations) {
		t.Fatalf("user_version = %d, want %d", got, len(migrations))
	}
}

func TestMigrateIsIdempotent(t *testing.T) {
	database := setupTestDB(t)
	ctx := context.Background()

	if err := database.Migrate(ctx); err != nil {
		t.Fatalf("Migrate(second run) error = %v", err)
	}

	if got := currentSchemaVersion(t, database, ctx); got != len(migrations) {
		t.Fatalf("user_version = %d, want %d", got, len(migrations))
	}
	if hasTable(t, database, ctx, "agents_new") {
		t.Fatal("agents_new should not remain after migration")
	}
}

func TestMigrateAddsReviewerToExistingTasks(t *testing.T) {
	database, err := OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory() error = %v", err)
	}
	t.Cleanup(func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	})

	ctx := context.Background()
	for i, stmt := range migrations[:len(migrations)-1] {
		if _, err := database.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("ExecContext(migration %d) error = %v", i, err)
		}
	}
	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("BeginTx() error = %v", err)
	}
	if err := setSchemaVersion(ctx, tx, len(migrations)-1); err != nil {
		t.Fatalf("setSchemaVersion() error = %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("Commit() error = %v", err)
	}
	if _, err := database.ExecContext(ctx, `INSERT INTO tasks (id, title, status, priority, blocked_by) VALUES ('tsk_legacy', 'legacy', 'pending', 'medium', '[]')`); err != nil {
		t.Fatalf("ExecContext(insert legacy task) error = %v", err)
	}

	if err := database.Migrate(ctx); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	task, err := database.FindTaskByID(ctx, "tsk_legacy")
	if err != nil {
		t.Fatalf("FindTaskByID() error = %v", err)
	}
	if task.Reviewer != "" {
		t.Fatalf("Reviewer = %q, want empty string", task.Reviewer)
	}
}

type pragmaColumn struct {
	notNull      int
	defaultValue string
}

func currentSchemaVersion(t *testing.T, database *DB, ctx context.Context) int {
	t.Helper()

	var version int
	if err := database.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&version); err != nil {
		t.Fatalf("QueryRowContext(PRAGMA user_version) error = %v", err)
	}
	return version
}

func agentColumns(t *testing.T, database *DB, ctx context.Context) map[string]pragmaColumn {
	t.Helper()
	return tableColumns(t, database, ctx, "agents")
}

func tableColumns(t *testing.T, database *DB, ctx context.Context, tableName string) map[string]pragmaColumn {
	t.Helper()

	rows, err := database.QueryContext(ctx, fmt.Sprintf(`PRAGMA table_info(%s)`, tableName))
	if err != nil {
		t.Fatalf("QueryContext(PRAGMA table_info) error = %v", err)
	}
	defer rows.Close()

	columns := make(map[string]pragmaColumn)
	for rows.Next() {
		var (
			cid        int
			name       string
			dataType   string
			notNull    int
			defaultVal any
			pk         int
		)
		if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultVal, &pk); err != nil {
			t.Fatalf("rows.Scan() error = %v", err)
		}
		columns[name] = pragmaColumn{notNull: notNull, defaultValue: fmt.Sprint(defaultVal)}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows.Err() error = %v", err)
	}

	return columns
}

func hasIndex(t *testing.T, database *DB, ctx context.Context, tableName string, want string) bool {
	t.Helper()

	rows, err := database.QueryContext(ctx, fmt.Sprintf(`PRAGMA index_list(%s)`, tableName))
	if err != nil {
		t.Fatalf("QueryContext(PRAGMA index_list) error = %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var seq int
		var name string
		var unique int
		var origin string
		var partial int
		if err := rows.Scan(&seq, &name, &unique, &origin, &partial); err != nil {
			t.Fatalf("rows.Scan() error = %v", err)
		}
		if name == want {
			return true
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows.Err() error = %v", err)
	}

	return false
}

func hasUniqueNameProjectIndex(t *testing.T, database *DB, ctx context.Context) bool {
	t.Helper()

	rows, err := database.QueryContext(ctx, `PRAGMA index_list(agents)`)
	if err != nil {
		t.Fatalf("QueryContext(PRAGMA index_list) error = %v", err)
	}
	defer rows.Close()

	indexNames := make([]string, 0)
	for rows.Next() {
		var seq int
		var name string
		var unique int
		var origin string
		var partial int
		if err := rows.Scan(&seq, &name, &unique, &origin, &partial); err != nil {
			t.Fatalf("rows.Scan() error = %v", err)
		}
		if unique != 1 {
			continue
		}
		indexNames = append(indexNames, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows.Err() error = %v", err)
	}
	if err := rows.Close(); err != nil {
		t.Fatalf("rows.Close() error = %v", err)
	}

	for _, name := range indexNames {
		infoRows, err := database.QueryContext(ctx, fmt.Sprintf(`PRAGMA index_info(%s)`, name))
		if err != nil {
			t.Fatalf("QueryContext(PRAGMA index_info) error = %v", err)
		}

		columns := make([]string, 0, 2)
		for infoRows.Next() {
			var seqno int
			var cid int
			var columnName string
			if err := infoRows.Scan(&seqno, &cid, &columnName); err != nil {
				infoRows.Close()
				t.Fatalf("infoRows.Scan() error = %v", err)
			}
			columns = append(columns, columnName)
		}
		if err := infoRows.Err(); err != nil {
			infoRows.Close()
			t.Fatalf("infoRows.Err() error = %v", err)
		}
		if err := infoRows.Close(); err != nil {
			t.Fatalf("infoRows.Close() error = %v", err)
		}

		if len(columns) == 2 && columns[0] == "name" && columns[1] == "project" {
			return true
		}
	}

	return false
}

func hasTable(t *testing.T, database *DB, ctx context.Context, tableName string) bool {
	t.Helper()

	var count int
	if err := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, tableName).Scan(&count); err != nil {
		t.Fatalf("QueryRowContext(sqlite_master) error = %v", err)
	}
	return count > 0
}

func legacySchemaStatements() []string {
	return []string{
		`CREATE TABLE agents (
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
	}
}
