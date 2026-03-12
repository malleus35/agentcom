package db

import (
	"context"
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
