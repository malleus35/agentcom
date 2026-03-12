package message

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNewEnvelopeAndMarshalRoundTrip(t *testing.T) {
	payload := json.RawMessage(`{"hello":"world"}`)
	env := NewEnvelope("sender", "receiver", "notification", "topic", payload)

	if !strings.HasPrefix(env.ID, "msg_") {
		t.Fatalf("ID = %q, want msg_ prefix", env.ID)
	}
	if env.From != "sender" || env.To != "receiver" {
		t.Fatalf("unexpected addressing: %+v", env)
	}

	data, err := env.Marshal()
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	roundTrip, err := UnmarshalEnvelope(data)
	if err != nil {
		t.Fatalf("UnmarshalEnvelope() error = %v", err)
	}
	if string(roundTrip.Payload) != string(payload) {
		t.Fatalf("Payload = %s, want %s", string(roundTrip.Payload), string(payload))
	}
}

func TestUnmarshalEnvelopeRejectsInvalidJSON(t *testing.T) {
	if _, err := UnmarshalEnvelope([]byte(`{`)); err == nil {
		t.Fatal("UnmarshalEnvelope() error = nil, want error")
	}
}
