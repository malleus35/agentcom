package agent

import (
	"context"
	"testing"
	"time"
)

func TestHeartbeatStopsAfterContextCancellation(t *testing.T) {
	registry, database, _ := setupRegistryTest(t)
	ctx := context.Background()

	agentRecord, err := registry.Register(ctx, "heartbeat-stop", "worker", nil, "", "project-a")
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	hb := NewHeartbeat(database, agentRecord.ID)
	hb.interval = 10 * time.Millisecond
	hbCtx, cancel := context.WithCancel(ctx)
	hb.Start(hbCtx)

	time.Sleep(30 * time.Millisecond)
	current, err := database.FindAgentByID(ctx, agentRecord.ID)
	if err != nil {
		t.Fatalf("FindAgentByID() error = %v", err)
	}
	first := current.LastHeartbeat
	cancel()
	time.Sleep(40 * time.Millisecond)
	current, err = database.FindAgentByID(ctx, agentRecord.ID)
	if err != nil {
		t.Fatalf("FindAgentByID() after cancel error = %v", err)
	}
	second := current.LastHeartbeat
	time.Sleep(40 * time.Millisecond)
	current, err = database.FindAgentByID(ctx, agentRecord.ID)
	if err != nil {
		t.Fatalf("FindAgentByID() final error = %v", err)
	}
	if !second.Equal(current.LastHeartbeat) {
		t.Fatalf("heartbeat kept updating after cancel: %v -> %v", second, current.LastHeartbeat)
	}
	if second.Before(first) {
		t.Fatalf("heartbeat moved backwards: %v -> %v", first, second)
	}
}
