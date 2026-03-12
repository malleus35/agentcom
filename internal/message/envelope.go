package message

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	gonanoid "github.com/matoous/go-nanoid/v2"
)

type Envelope struct {
	ID            string          `json:"id"`
	From          string          `json:"from"`
	To            string          `json:"to,omitempty"`
	Type          string          `json:"type"`
	Topic         string          `json:"topic,omitempty"`
	Payload       json.RawMessage `json:"payload"`
	CorrelationID string          `json:"correlation_id,omitempty"`
	Timestamp     string          `json:"timestamp"`
}

// NewEnvelope creates a new envelope with generated ID and UTC timestamp.
func NewEnvelope(from, to, msgType, topic string, payload json.RawMessage) *Envelope {
	id, err := gonanoid.New()
	if err != nil {
		id = strconv.FormatInt(time.Now().UTC().UnixNano(), 10)
	}

	return &Envelope{
		ID:        "msg_" + id,
		From:      from,
		To:        to,
		Type:      msgType,
		Topic:     topic,
		Payload:   payload,
		Timestamp: time.Now().UTC().Format(time.DateTime),
	}
}

// Marshal serializes the envelope into JSON.
func (e *Envelope) Marshal() ([]byte, error) {
	b, err := json.Marshal(e)
	if err != nil {
		return nil, fmt.Errorf("message.Envelope.Marshal: %w", err)
	}

	return b, nil
}

// UnmarshalEnvelope deserializes JSON data into an envelope.
func UnmarshalEnvelope(data []byte) (*Envelope, error) {
	var e Envelope
	if err := json.Unmarshal(data, &e); err != nil {
		return nil, fmt.Errorf("message.UnmarshalEnvelope: %w", err)
	}

	return &e, nil
}
