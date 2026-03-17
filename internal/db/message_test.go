package db

import (
	"context"
	"errors"
	"testing"
	"time"
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

func TestListMessagesFromAgent(t *testing.T) {
	database := setupTestDB(t)
	ctx := context.Background()

	sender := &Agent{Name: "sender", Type: "worker", Project: "project-a", Status: "alive"}
	receiver := &Agent{Name: "receiver", Type: "worker", Project: "project-a", Status: "alive"}
	other := &Agent{Name: "other", Type: "worker", Project: "project-a", Status: "alive"}
	for _, agent := range []*Agent{sender, receiver, other} {
		if err := database.InsertAgent(ctx, agent); err != nil {
			t.Fatalf("InsertAgent(%s) error = %v", agent.Name, err)
		}
	}

	first := &Message{FromAgent: sender.ID, ToAgent: receiver.ID, Type: "response", Payload: `{"text":"first"}`, CreatedAt: "2026-03-17 10:00:00"}
	second := &Message{FromAgent: sender.ID, ToAgent: other.ID, Type: "response", Payload: `{"text":"second"}`, CreatedAt: "2026-03-17 10:01:00"}
	third := &Message{FromAgent: other.ID, ToAgent: receiver.ID, Type: "response", Payload: `{"text":"third"}`, CreatedAt: "2026-03-17 10:02:00"}
	for _, msg := range []*Message{first, second, third} {
		if err := database.InsertMessage(ctx, msg); err != nil {
			t.Fatalf("InsertMessage(%s) error = %v", msg.Payload, err)
		}
	}

	got, err := database.ListMessagesFromAgent(ctx, sender.ID)
	if err != nil {
		t.Fatalf("ListMessagesFromAgent() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(ListMessagesFromAgent()) = %d, want 2", len(got))
	}
	if got[0].ID != second.ID || got[1].ID != first.ID {
		t.Fatalf("ListMessagesFromAgent() order = [%s %s], want [%s %s]", got[0].ID, got[1].ID, second.ID, first.ID)
	}

	empty, err := database.ListMessagesFromAgent(ctx, "missing")
	if err != nil {
		t.Fatalf("ListMessagesFromAgent(missing) error = %v", err)
	}
	if len(empty) != 0 {
		t.Fatalf("len(ListMessagesFromAgent(missing)) = %d, want 0", len(empty))
	}
}

func TestListUnreadRequestsForAgent(t *testing.T) {
	database := setupTestDB(t)
	ctx := context.Background()

	sender := &Agent{Name: "sender", Type: "worker", Project: "project-a", Status: "alive"}
	user := &Agent{Name: "user", Type: "human", Project: "project-a", Status: "alive"}
	for _, agent := range []*Agent{sender, user} {
		if err := database.InsertAgent(ctx, agent); err != nil {
			t.Fatalf("InsertAgent(%s) error = %v", agent.Name, err)
		}
	}

	requestOld := &Message{FromAgent: sender.ID, ToAgent: user.ID, Type: "request", Payload: `{"text":"old"}`, CreatedAt: "2026-03-17 10:00:00"}
	requestNew := &Message{FromAgent: sender.ID, ToAgent: user.ID, Type: "request", Payload: `{"text":"new"}`, CreatedAt: "2026-03-17 10:01:00"}
	response := &Message{FromAgent: sender.ID, ToAgent: user.ID, Type: "response", Payload: `{"text":"ignore"}`, CreatedAt: "2026-03-17 10:02:00"}
	readRequest := &Message{FromAgent: sender.ID, ToAgent: user.ID, Type: "request", Payload: `{"text":"read"}`, CreatedAt: "2026-03-17 10:03:00", ReadAt: time.Now().UTC().Format(time.DateTime)}
	for _, msg := range []*Message{requestOld, requestNew, response, readRequest} {
		if err := database.InsertMessage(ctx, msg); err != nil {
			t.Fatalf("InsertMessage(%s) error = %v", msg.Payload, err)
		}
	}

	got, err := database.ListUnreadRequestsForAgent(ctx, user.ID)
	if err != nil {
		t.Fatalf("ListUnreadRequestsForAgent() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(ListUnreadRequestsForAgent()) = %d, want 2", len(got))
	}
	if got[0].ID != requestNew.ID || got[1].ID != requestOld.ID {
		t.Fatalf("ListUnreadRequestsForAgent() order = [%s %s], want [%s %s]", got[0].ID, got[1].ID, requestNew.ID, requestOld.ID)
	}
}
