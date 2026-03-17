package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/malleus35/agentcom/internal/config"
)

type upRuntimeState struct {
	Project       string                `json:"project,omitempty"`
	ProjectDir    string                `json:"project_dir"`
	Template      string                `json:"template"`
	StartedAt     time.Time             `json:"started_at"`
	SupervisorPID int                   `json:"supervisor_pid"`
	UserAgent     *upRuntimeStateAgent  `json:"user_agent,omitempty"`
	Agents        []upRuntimeStateAgent `json:"agents"`
}

type upRuntimeStateAgent struct {
	Role       string `json:"role"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	PID        int    `json:"pid"`
	AgentID    string `json:"agent_id,omitempty"`
	SocketPath string `json:"socket_path,omitempty"`
	Project    string `json:"project,omitempty"`
}

func upRuntimeStatePath(projectDir string) string {
	return filepath.Join(projectDir, ".agentcom", config.RunDir, "up.json")
}

func writeUpRuntimeState(projectDir string, state upRuntimeState) error {
	path := upRuntimeStatePath(projectDir)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("cli.writeUpRuntimeState: mkdir: %w", err)
	}
	sort.Slice(state.Agents, func(i, j int) bool {
		return state.Agents[i].Role < state.Agents[j].Role
	})
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("cli.writeUpRuntimeState: marshal: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("cli.writeUpRuntimeState: write: %w", err)
	}
	return nil
}

func loadUpRuntimeState(projectDir string) (upRuntimeState, string, error) {
	path := upRuntimeStatePath(projectDir)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return upRuntimeState{}, path, nil
		}
		return upRuntimeState{}, path, fmt.Errorf("cli.loadUpRuntimeState: read: %w", err)
	}
	var state upRuntimeState
	if err := json.Unmarshal(data, &state); err != nil {
		return upRuntimeState{}, path, fmt.Errorf("cli.loadUpRuntimeState: decode: %w", err)
	}
	return state, path, nil
}

func removeUpRuntimeState(projectDir string) error {
	path := upRuntimeStatePath(projectDir)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("cli.removeUpRuntimeState: %w", err)
	}
	return nil
}
