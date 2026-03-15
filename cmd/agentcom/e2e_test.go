package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

type registerProcess struct {
	cmd    *exec.Cmd
	stdout io.ReadCloser
	stderr *bytes.Buffer
	meta   map[string]any
}

func TestAgentcomE2EFlow(t *testing.T) {
	binPath := buildAgentcomBinary(t)
	homeDir := t.TempDir()
	projectDir := t.TempDir()

	initOutput := runAgentcomJSON(t, binPath, homeDir, projectDir, "init", "--agents-md")
	if initOutput["agents_md"] == "" {
		t.Fatalf("init output missing agents_md: %#v", initOutput)
	}
	if _, err := os.Stat(filepath.Join(projectDir, "AGENTS.md")); err != nil {
		t.Fatalf("AGENTS.md not created: %v", err)
	}

	alpha := startRegisterProcess(t, binPath, homeDir, "alpha", "executor")
	defer stopRegisterProcess(t, alpha)
	beta := startRegisterProcess(t, binPath, homeDir, "beta", "reviewer")
	defer stopRegisterProcess(t, beta)

	waitForAgents(t, binPath, homeDir, 2)

	runAgentcomJSON(t, binPath, homeDir, "", "send", "--from", "alpha", "beta", `{"text":"hello"}`)

	inbox := runAgentcomJSONArray(t, binPath, homeDir, "", "inbox", "--agent", "beta", "--unread")
	if len(inbox) == 0 {
		t.Fatal("beta inbox is empty after send")
	}

	createdTask := runAgentcomJSON(t, binPath, homeDir, "", "task", "create", "Wave 9 task", "--creator", "alpha")
	taskID, _ := createdTask["id"].(string)
	if taskID == "" {
		t.Fatalf("task create output missing id: %#v", createdTask)
	}

	delegated := runAgentcomJSON(t, binPath, homeDir, "", "task", "delegate", taskID, "--to", "beta")
	if delegated["delegated_to"] != beta.meta["id"] {
		t.Fatalf("delegated_to = %v, want %v", delegated["delegated_to"], beta.meta["id"])
	}

	status := runAgentcomJSON(t, binPath, homeDir, "", "status")
	if got := intFromMap(t, status, "total_agents"); got != 2 {
		t.Fatalf("total_agents = %d, want 2", got)
	}
	if got := intFromMap(t, status, "total_messages"); got < 1 {
		t.Fatalf("total_messages = %d, want >= 1", got)
	}
	if got := intFromMap(t, status, "total_tasks"); got != 1 {
		t.Fatalf("total_tasks = %d, want 1", got)
	}

	stopRegisterProcess(t, alpha)
	alpha = nil
	stopRegisterProcess(t, beta)
	beta = nil

	waitForAgents(t, binPath, homeDir, 0)
	agents := runAgentcomJSONArray(t, binPath, homeDir, "", "list")
	if len(agents) != 0 {
		t.Fatalf("remaining agents = %d, want 0", len(agents))
	}
}

func TestAgentcomUpDownFlow(t *testing.T) {
	binPath := buildAgentcomBinary(t)
	homeDir := t.TempDir()
	projectDir := t.TempDir()

	initOutput := runAgentcomJSON(t, binPath, homeDir, projectDir, "init", "--template", "company")
	if initOutput["active_template"] != "company" {
		t.Fatalf("active_template = %v, want company", initOutput["active_template"])
	}

	upOutput := runAgentcomJSON(t, binPath, homeDir, projectDir, "up", "--only", "frontend,plan")
	if upOutput["template"] != "company" {
		t.Fatalf("template = %v, want company", upOutput["template"])
	}
	agents, ok := upOutput["agents"].([]any)
	if !ok || len(agents) != 2 {
		t.Fatalf("agents = %#v, want 2 entries", upOutput["agents"])
	}
	if _, err := os.Stat(filepath.Join(projectDir, ".agentcom", "run", "up.json")); err != nil {
		t.Fatalf("runtime state missing: %v", err)
	}

	waitForAgents(t, binPath, homeDir, 2)

	downOutput := runAgentcomJSON(t, binPath, homeDir, projectDir, "down")
	stoppedRoles, ok := downOutput["stopped_roles"].([]any)
	if !ok || len(stoppedRoles) != 2 {
		t.Fatalf("stopped_roles = %#v, want 2 entries", downOutput["stopped_roles"])
	}

	waitForAgents(t, binPath, homeDir, 0)
	if _, err := os.Stat(filepath.Join(projectDir, ".agentcom", "run", "up.json")); !os.IsNotExist(err) {
		t.Fatalf("runtime state should be removed, stat err=%v", err)
	}
}

func buildAgentcomBinary(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller() failed")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	binPath := filepath.Join(t.TempDir(), "agentcom-test-bin")

	cmd := exec.Command("go", "build", "-o", binPath, "./cmd/agentcom")
	cmd.Dir = repoRoot
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build failed: %v\n%s", err, string(output))
	}

	return binPath
}

func startRegisterProcess(t *testing.T, binPath, homeDir, name, agentType string) *registerProcess {
	t.Helper()

	cmd := exec.Command(binPath, "--json", "register", "--name", name, "--type", agentType)
	cmd.Env = append(os.Environ(), "AGENTCOM_HOME="+homeDir)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("StdoutPipe() error = %v", err)
	}
	stderr := &bytes.Buffer{}
	cmd.Stderr = stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("register start error = %v", err)
	}

	dec := json.NewDecoder(stdout)
	meta := map[string]any{}
	if err := dec.Decode(&meta); err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		t.Fatalf("decode register output error = %v stderr=%s", err, stderr.String())
	}

	return &registerProcess{cmd: cmd, stdout: stdout, stderr: stderr, meta: meta}
}

func stopRegisterProcess(t *testing.T, proc *registerProcess) {
	t.Helper()
	if proc == nil || proc.cmd == nil || proc.cmd.Process == nil {
		return
	}

	_ = proc.cmd.Process.Signal(os.Interrupt)
	done := make(chan error, 1)
	go func() {
		done <- proc.cmd.Wait()
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("register process exit error = %v stderr=%s", err, proc.stderr.String())
		}
	case <-time.After(5 * time.Second):
		_ = proc.cmd.Process.Kill()
		_ = proc.cmd.Wait()
		t.Fatalf("timed out stopping register process stderr=%s", proc.stderr.String())
	}

	proc.cmd = nil
}

func waitForAgents(t *testing.T, binPath, homeDir string, want int) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		agents := runAgentcomJSONArray(t, binPath, homeDir, "", "list")
		if len(agents) == want {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}

	agents := runAgentcomJSONArray(t, binPath, homeDir, "", "list")
	t.Fatalf("agent count = %d, want %d", len(agents), want)
}

func runAgentcomJSON(t *testing.T, binPath, homeDir, dir string, args ...string) map[string]any {
	t.Helper()

	fullArgs := append([]string{"--json"}, args...)
	cmd := exec.Command(binPath, fullArgs...)
	cmd.Env = append(os.Environ(), "AGENTCOM_HOME="+homeDir)
	if dir != "" {
		cmd.Dir = dir
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s failed: %v\n%s", fmt.Sprintf("%v", args), err, string(output))
	}

	var parsed map[string]any
	if err := json.Unmarshal(output, &parsed); err != nil {
		t.Fatalf("json.Unmarshal(%v) error = %v output=%s", args, err, string(output))
	}

	return parsed
}

func runAgentcomJSONArray(t *testing.T, binPath, homeDir, dir string, args ...string) []map[string]any {
	t.Helper()

	fullArgs := append([]string{"--json"}, args...)
	cmd := exec.Command(binPath, fullArgs...)
	cmd.Env = append(os.Environ(), "AGENTCOM_HOME="+homeDir)
	if dir != "" {
		cmd.Dir = dir
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s failed: %v\n%s", fmt.Sprintf("%v", args), err, string(output))
	}

	var parsed []map[string]any
	if err := json.Unmarshal(output, &parsed); err != nil {
		t.Fatalf("json.Unmarshal(%v) error = %v output=%s", args, err, string(output))
	}

	return parsed
}

func intFromMap(t *testing.T, m map[string]any, key string) int {
	t.Helper()

	v, ok := m[key].(float64)
	if !ok {
		t.Fatalf("map[%q] type = %T, want number", key, m[key])
	}

	return int(v)
}
