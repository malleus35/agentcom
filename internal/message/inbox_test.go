package message

import (
	"context"
	"testing"

	"github.com/malleus35/agentcom/internal/db"
)

func TestInboxOperations(t *testing.T) {
	database := setupMessageTestDB(t)
	inbox := NewInbox(database)
	ctx := context.Background()

	msg1 := &db.Message{FromAgent: "agt_sender", ToAgent: "agt_receiver", Type: "notification", Payload: `{}`, CorrelationID: "corr-1"}
	msg2 := &db.Message{FromAgent: "agt_sender", ToAgent: "agt_receiver", Type: "notification", Payload: `{}`, CorrelationID: "corr-1"}
	if err := database.InsertMessage(ctx, msg1); err != nil {
		t.Fatalf("InsertMessage(msg1) error = %v", err)
	}
	if err := database.InsertMessage(ctx, msg2); err != nil {
		t.Fatalf("InsertMessage(msg2) error = %v", err)
	}

	all, err := inbox.ListMessages(ctx, "agt_receiver")
	if err != nil {
		t.Fatalf("ListMessages() error = %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("len(ListMessages()) = %d, want 2", len(all))
	}

	unread, err := inbox.ListUnread(ctx, "agt_receiver")
	if err != nil {
		t.Fatalf("ListUnread() error = %v", err)
	}
	if len(unread) != 2 {
		t.Fatalf("len(ListUnread()) = %d, want 2", len(unread))
	}

	if err := inbox.MarkRead(ctx, msg1.ID); err != nil {
		t.Fatalf("MarkRead() error = %v", err)
	}

	unread, err = inbox.ListUnread(ctx, "agt_receiver")
	if err != nil {
		t.Fatalf("ListUnread(after read) error = %v", err)
	}
	if len(unread) != 1 {
		t.Fatalf("len(ListUnread()) after read = %d, want 1", len(unread))
	}

	correlated, err := inbox.ListByCorrelation(ctx, "corr-1")
	if err != nil {
		t.Fatalf("ListByCorrelation() error = %v", err)
	}
	if len(correlated) != 2 {
		t.Fatalf("len(ListByCorrelation()) = %d, want 2", len(correlated))
	}
}
