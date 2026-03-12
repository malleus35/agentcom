package db

import (
	"context"
	"errors"
	"testing"
)

func TestMessageCRUD(t *testing.T) {
	database := setupTestDB(t)
	ctx := context.Background()

	sender := &Agent{Name: "sender", Type: "worker", Status: "alive"}
	if err := database.InsertAgent(ctx, sender); err != nil {
		t.Fatalf("InsertAgent(sender) error = %v", err)
	}
	receiver := &Agent{Name: "receiver", Type: "worker", Status: "alive"}
	if err := database.InsertAgent(ctx, receiver); err != nil {
		t.Fatalf("InsertAgent(receiver) error = %v", err)
	}

	direct := &Message{
		FromAgent:     sender.ID,
		ToAgent:       receiver.ID,
		Type:          "notification",
		Topic:         "direct",
		Payload:       `{"ok":true}`,
		CorrelationID: "corr-1",
	}
	if err := database.InsertMessage(ctx, direct); err != nil {
		t.Fatalf("InsertMessage(direct) error = %v", err)
	}

	broadcast := &Message{
		FromAgent:     sender.ID,
		Type:          "event",
		Topic:         "broadcast",
		Payload:       `{"all":true}`,
		CorrelationID: "corr-1",
	}
	if err := database.InsertMessage(ctx, broadcast); err != nil {
		t.Fatalf("InsertMessage(broadcast) error = %v", err)
	}

	got, err := database.FindMessageByID(ctx, direct.ID)
	if err != nil {
		t.Fatalf("FindMessageByID() error = %v", err)
	}
	if got.Topic != "direct" {
		t.Fatalf("Topic = %q, want direct", got.Topic)
	}

	messages, err := database.ListMessagesForAgent(ctx, receiver.ID)
	if err != nil {
		t.Fatalf("ListMessagesForAgent() error = %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("len(ListMessagesForAgent()) = %d, want 2", len(messages))
	}

	unread, err := database.ListUnreadMessages(ctx, receiver.ID)
	if err != nil {
		t.Fatalf("ListUnreadMessages() error = %v", err)
	}
	if len(unread) != 2 {
		t.Fatalf("len(ListUnreadMessages()) = %d, want 2", len(unread))
	}

	if err := database.MarkDelivered(ctx, direct.ID); err != nil {
		t.Fatalf("MarkDelivered() error = %v", err)
	}
	if err := database.MarkRead(ctx, direct.ID); err != nil {
		t.Fatalf("MarkRead() error = %v", err)
	}

	got, err = database.FindMessageByID(ctx, direct.ID)
	if err != nil {
		t.Fatalf("FindMessageByID(after marks) error = %v", err)
	}
	if got.DeliveredAt == "" || got.ReadAt == "" {
		t.Fatalf("message timestamps not updated: %+v", got)
	}

	correlated, err := database.ListByCorrelation(ctx, "corr-1")
	if err != nil {
		t.Fatalf("ListByCorrelation() error = %v", err)
	}
	if len(correlated) != 2 {
		t.Fatalf("len(ListByCorrelation()) = %d, want 2", len(correlated))
	}

	if err := database.MarkRead(ctx, "missing"); !errors.Is(err, ErrMessageNotFound) {
		t.Fatalf("MarkRead(missing) error = %v, want %v", err, ErrMessageNotFound)
	}
}
