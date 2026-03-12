package message

import (
	"context"
	"fmt"

	"github.com/malleus35/agentcom/internal/db"
)

// Inbox provides read/query operations for agent inbox messages.
type Inbox struct {
	db *db.DB
}

// NewInbox creates a new inbox service.
func NewInbox(database *db.DB) *Inbox {
	return &Inbox{db: database}
}

// ListMessages returns all messages for a target agent.
func (i *Inbox) ListMessages(ctx context.Context, agentID string) ([]*db.Message, error) {
	messages, err := i.db.ListMessagesForAgent(ctx, agentID)
	if err != nil {
		return nil, fmt.Errorf("message.Inbox.ListMessages: %w", err)
	}
	return messages, nil
}

// ListUnread returns unread messages for a target agent.
func (i *Inbox) ListUnread(ctx context.Context, agentID string) ([]*db.Message, error) {
	messages, err := i.db.ListUnreadMessages(ctx, agentID)
	if err != nil {
		return nil, fmt.Errorf("message.Inbox.ListUnread: %w", err)
	}
	return messages, nil
}

// MarkRead marks a message as read.
func (i *Inbox) MarkRead(ctx context.Context, messageID string) error {
	if err := i.db.MarkRead(ctx, messageID); err != nil {
		return fmt.Errorf("message.Inbox.MarkRead: %w", err)
	}
	return nil
}

// ListByCorrelation returns messages linked by correlation ID.
func (i *Inbox) ListByCorrelation(ctx context.Context, correlationID string) ([]*db.Message, error) {
	messages, err := i.db.ListByCorrelation(ctx, correlationID)
	if err != nil {
		return nil, fmt.Errorf("message.Inbox.ListByCorrelation: %w", err)
	}
	return messages, nil
}
