package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestBuildPayload(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "json object", input: `{"ok":true}`, want: `{"ok":true}`},
		{name: "json array", input: `[1,2,3]`, want: `[1,2,3]`},
		{name: "plain text", input: `hello`, want: `{"text":"hello"}`},
		{name: "invalid json", input: `{bad`, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload, err := buildPayload(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("buildPayload() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("buildPayload() error = %v", err)
			}
			if string(payload) != tt.want {
				t.Fatalf("payload = %s, want %s", string(payload), tt.want)
			}
		})
	}
}

func TestParseCapabilities(t *testing.T) {
	got := parseCapabilities("plan, execute, ,test")
	want := []string{"plan", "execute", "test"}
	if len(got) != len(want) {
		t.Fatalf("len(parseCapabilities()) = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestSplitCSVAndNullIfEmpty(t *testing.T) {
	parts := splitCSV("a, b, ,c")
	if strings.Join(parts, ",") != "a,b,c" {
		t.Fatalf("splitCSV() = %v, want [a b c]", parts)
	}
	if nullIfEmpty("") != nil {
		t.Fatal("nullIfEmpty(empty) should return nil")
	}
	if got := nullIfEmpty("value"); got != "value" {
		t.Fatalf("nullIfEmpty(value) = %#v, want value", got)
	}
}

func TestParseTimestamp(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "rfc3339", input: "2026-03-12T10:20:30Z"},
		{name: "datetime", input: "2026-03-12 10:20:30"},
		{name: "empty", input: "", wantErr: true},
		{name: "invalid", input: "not-a-time", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseTimestamp(tt.input)
			if tt.wantErr && err == nil {
				t.Fatal("parseTimestamp() error = nil, want error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("parseTimestamp() error = %v", err)
			}
		})
	}
}

func TestVersionCommandOutputsJSON(t *testing.T) {
	oldJSON := jsonOutput
	jsonOutput = true
	defer func() { jsonOutput = oldJSON }()

	Version = "1.2.3"
	BuildDate = "2026-03-12T00:00:00Z"
	GoVersion = "go1.24"

	buf := &bytes.Buffer{}
	cmd := newVersionCmd()
	cmd.SetOut(buf)
	cmd.SetArgs([]string{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute() error = %v", err)
	}

	var got map[string]string
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v output=%s", err, buf.String())
	}
	if got["version"] != "1.2.3" {
		t.Fatalf("version = %q, want 1.2.3", got["version"])
	}
}

func TestRootCommandContainsCoreSubcommands(t *testing.T) {
	root := NewRootCmd()
	want := []string{"init", "agents", "skill", "register", "send", "task", "mcp-server", "version"}
	for _, name := range want {
		if _, _, err := root.Find([]string{name}); err != nil {
			t.Fatalf("root.Find(%q) error = %v", name, err)
		}
	}
}
