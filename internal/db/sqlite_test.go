package db

import (
	"context"
	"path/filepath"
	"testing"
)

func setupTestDB(t *testing.T) *DB {
	t.Helper()

	database, err := OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory() error = %v", err)
	}

	t.Cleanup(func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	})

	if err := database.Migrate(context.Background()); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	return database
}

func TestHealthCheckReportsSQLiteState(t *testing.T) {
	database, err := Open(filepath.Join(t.TempDir(), "agentcom.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	})

	status, err := database.HealthCheck(context.Background())
	if err != nil {
		t.Fatalf("HealthCheck() error = %v", err)
	}
	if status.IntegrityCheck != "ok" {
		t.Fatalf("IntegrityCheck = %q, want ok", status.IntegrityCheck)
	}
	if status.JournalMode != "wal" {
		t.Fatalf("JournalMode = %q, want wal", status.JournalMode)
	}
}

func TestHealthCheckReturnsErrorAfterClose(t *testing.T) {
	database, err := OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory() error = %v", err)
	}
	if err := database.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	if _, err := database.HealthCheck(context.Background()); err == nil {
		t.Fatal("HealthCheck() error = nil, want error")
	}
}
