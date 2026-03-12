package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	gonanoid "github.com/matoous/go-nanoid/v2"
)

var ErrMessageNotFound = errors.New("message not found")

// Message represents a message persisted in SQLite.
type Message struct {
	ID            string
	FromAgent     string
	ToAgent       string
	Type          string
	Topic         string
	Payload       string
	CorrelationID string
	CreatedAt     string
	DeliveredAt   string
	ReadAt        string
}

// InsertMessage inserts a new message row and generates a message ID.
func (d *DB) InsertMessage(ctx context.Context, msg *Message) error {
	id, err := gonanoid.New()
	if err != nil {
		return fmt.Errorf("db.InsertMessage: generate id: %w", err)
	}
	msg.ID = "msg_" + id

	stmt, err := d.PrepareContext(ctx, `
		INSERT INTO messages (
			id, from_agent, to_agent, type, topic, payload, correlation_id
		) VALUES (?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("db.InsertMessage: prepare: %w", err)
	}
	defer stmt.Close()

	if _, err := stmt.ExecContext(ctx,
		msg.ID,
		msg.FromAgent,
		nullableString(msg.ToAgent),
		msg.Type,
		nullableString(msg.Topic),
		msg.Payload,
		nullableString(msg.CorrelationID),
	); err != nil {
		return fmt.Errorf("db.InsertMessage: exec: %w", err)
	}

	return nil
}

// FindMessageByID finds a message by ID.
func (d *DB) FindMessageByID(ctx context.Context, id string) (*Message, error) {
	stmt, err := d.PrepareContext(ctx, `
		SELECT
			id, from_agent, to_agent, type, topic, payload, correlation_id, created_at, delivered_at, read_at
		FROM messages
		WHERE id = ?
	`)
	if err != nil {
		return nil, fmt.Errorf("db.FindMessageByID: prepare: %w", err)
	}
	defer stmt.Close()

	msg, err := scanMessage(stmt.QueryRowContext(ctx, id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrMessageNotFound
		}
		return nil, fmt.Errorf("db.FindMessageByID: %w", err)
	}

	return msg, nil
}

// ListMessagesForAgent lists direct and broadcast messages for an agent.
func (d *DB) ListMessagesForAgent(ctx context.Context, agentID string) ([]*Message, error) {
	stmt, err := d.PrepareContext(ctx, `
		SELECT
			id, from_agent, to_agent, type, topic, payload, correlation_id, created_at, delivered_at, read_at
		FROM messages
		WHERE to_agent = ? OR to_agent IS NULL
		ORDER BY created_at ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("db.ListMessagesForAgent: prepare: %w", err)
	}
	defer stmt.Close()

	rows, err := stmt.QueryContext(ctx, agentID)
	if err != nil {
		return nil, fmt.Errorf("db.ListMessagesForAgent: query: %w", err)
	}
	defer rows.Close()

	messages := make([]*Message, 0)
	for rows.Next() {
		msg, err := scanMessage(rows)
		if err != nil {
			return nil, fmt.Errorf("db.ListMessagesForAgent: scan: %w", err)
		}
		messages = append(messages, msg)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db.ListMessagesForAgent: rows: %w", err)
	}

	return messages, nil
}

// ListUnreadMessages lists unread direct and broadcast messages for an agent.
func (d *DB) ListUnreadMessages(ctx context.Context, agentID string) ([]*Message, error) {
	stmt, err := d.PrepareContext(ctx, `
		SELECT
			id, from_agent, to_agent, type, topic, payload, correlation_id, created_at, delivered_at, read_at
		FROM messages
		WHERE (to_agent = ? OR to_agent IS NULL) AND read_at IS NULL
		ORDER BY created_at ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("db.ListUnreadMessages: prepare: %w", err)
	}
	defer stmt.Close()

	rows, err := stmt.QueryContext(ctx, agentID)
	if err != nil {
		return nil, fmt.Errorf("db.ListUnreadMessages: query: %w", err)
	}
	defer rows.Close()

	messages := make([]*Message, 0)
	for rows.Next() {
		msg, err := scanMessage(rows)
		if err != nil {
			return nil, fmt.Errorf("db.ListUnreadMessages: scan: %w", err)
		}
		messages = append(messages, msg)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db.ListUnreadMessages: rows: %w", err)
	}

	return messages, nil
}

// MarkDelivered marks a message as delivered.
func (d *DB) MarkDelivered(ctx context.Context, id string) error {
	stmt, err := d.PrepareContext(ctx, `
		UPDATE messages
		SET delivered_at = datetime('now')
		WHERE id = ?
	`)
	if err != nil {
		return fmt.Errorf("db.MarkDelivered: prepare: %w", err)
	}
	defer stmt.Close()

	res, err := stmt.ExecContext(ctx, id)
	if err != nil {
		return fmt.Errorf("db.MarkDelivered: exec: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("db.MarkDelivered: rows affected: %w", err)
	}
	if rows == 0 {
		return ErrMessageNotFound
	}

	return nil
}

// MarkRead marks a message as read.
func (d *DB) MarkRead(ctx context.Context, id string) error {
	stmt, err := d.PrepareContext(ctx, `
		UPDATE messages
		SET read_at = datetime('now')
		WHERE id = ?
	`)
	if err != nil {
		return fmt.Errorf("db.MarkRead: prepare: %w", err)
	}
	defer stmt.Close()

	res, err := stmt.ExecContext(ctx, id)
	if err != nil {
		return fmt.Errorf("db.MarkRead: exec: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("db.MarkRead: rows affected: %w", err)
	}
	if rows == 0 {
		return ErrMessageNotFound
	}

	return nil
}

// ListByCorrelation lists messages with the same correlation ID.
func (d *DB) ListByCorrelation(ctx context.Context, correlationID string) ([]*Message, error) {
	stmt, err := d.PrepareContext(ctx, `
		SELECT
			id, from_agent, to_agent, type, topic, payload, correlation_id, created_at, delivered_at, read_at
		FROM messages
		WHERE correlation_id = ?
		ORDER BY created_at ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("db.ListByCorrelation: prepare: %w", err)
	}
	defer stmt.Close()

	rows, err := stmt.QueryContext(ctx, correlationID)
	if err != nil {
		return nil, fmt.Errorf("db.ListByCorrelation: query: %w", err)
	}
	defer rows.Close()

	messages := make([]*Message, 0)
	for rows.Next() {
		msg, err := scanMessage(rows)
		if err != nil {
			return nil, fmt.Errorf("db.ListByCorrelation: scan: %w", err)
		}
		messages = append(messages, msg)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db.ListByCorrelation: rows: %w", err)
	}

	return messages, nil
}

func scanMessage(scanner rowScanner) (*Message, error) {
	msg := &Message{}
	var toAgent sql.NullString
	var topic sql.NullString
	var correlationID sql.NullString
	var deliveredAt sql.NullString
	var readAt sql.NullString

	if err := scanner.Scan(
		&msg.ID,
		&msg.FromAgent,
		&toAgent,
		&msg.Type,
		&topic,
		&msg.Payload,
		&correlationID,
		&msg.CreatedAt,
		&deliveredAt,
		&readAt,
	); err != nil {
		return nil, err
	}

	if toAgent.Valid {
		msg.ToAgent = toAgent.String
	}
	if topic.Valid {
		msg.Topic = topic.String
	}
	if correlationID.Valid {
		msg.CorrelationID = correlationID.String
	}
	if deliveredAt.Valid {
		msg.DeliveredAt = deliveredAt.String
	}
	if readAt.Valid {
		msg.ReadAt = readAt.String
	}

	return msg, nil
}
