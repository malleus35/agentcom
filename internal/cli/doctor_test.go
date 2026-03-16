package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

type doctorCheckPayload struct {
	Category string `json:"category"`
	Name     string `json:"name"`
	Status   string `json:"status"`
	Message  string `json:"message"`
	Fix      string `json:"fix,omitempty"`
}

func TestRootCommandContainsDoctorSubcommand(t *testing.T) {
	root := NewRootCmd()
	if _, _, err := root.Find([]string{"doctor"}); err != nil {
		t.Fatalf("root.Find(doctor) error = %v", err)
	}
}

func TestDoctorOnEmptyProject(t *testing.T) {
	projectDir := t.TempDir()
	homeDir := filepath.Join(t.TempDir(), "agentcom-home")
	output := executeRootCommandInDir(t, projectDir, homeDir, []string{"--json", "doctor"})

	checks := decodeDoctorChecks(t, output)
	if len(checks) == 0 {
		t.Fatal("doctor returned no checks")
	}
	if !hasDoctorStatus(checks, "project", "fail") {
		t.Fatalf("doctor checks = %#v, want at least one failing project check", checks)
	}
	if !hasDoctorStatus(checks, "environment", "pass") {
		t.Fatalf("doctor checks = %#v, want passing environment checks", checks)
	}
	if !hasDoctorFix(checks) {
		t.Fatalf("doctor checks = %#v, want at least one actionable fix", checks)
	}
}

func TestDoctorOnInitializedProject(t *testing.T) {
	projectDir := t.TempDir()
	homeDir := filepath.Join(t.TempDir(), "agentcom-home")
	_ = executeRootCommandInDir(t, projectDir, homeDir, []string{"init", "--batch", "--project", "demo-app"})
	output := executeRootCommandInDir(t, projectDir, homeDir, []string{"--json", "doctor"})

	checks := decodeDoctorChecks(t, output)
	if !hasNamedDoctorStatus(checks, "project", "project_config", "pass") {
		t.Fatalf("doctor checks = %#v, want project_config pass", checks)
	}
	if !hasNamedDoctorStatus(checks, "project", "active_template", "fail") {
		t.Fatalf("doctor checks = %#v, want active_template fail without scaffold", checks)
	}
}

func TestDoctorOnFullTemplate(t *testing.T) {
	projectDir := t.TempDir()
	homeDir := filepath.Join(t.TempDir(), "agentcom-home")
	_ = executeRootCommandInDir(t, projectDir, homeDir, []string{"init", "--batch", "--project", "demo-app", "--template", "company"})
	output := executeRootCommandInDir(t, projectDir, homeDir, []string{"--json", "doctor"})

	checks := decodeDoctorChecks(t, output)
	for _, check := range checks {
		if check.Status == "fail" {
			t.Fatalf("doctor check failed in full template setup: %#v", check)
		}
	}
	if !hasNamedDoctorStatus(checks, "documentation", "shared_skills", "pass") {
		t.Fatalf("doctor checks = %#v, want shared_skills pass", checks)
	}
	if !hasNamedDoctorStatus(checks, "communication", "template_graph", "pass") {
		t.Fatalf("doctor checks = %#v, want template_graph pass", checks)
	}
}

func executeRootCommandInDir(t *testing.T, projectDir string, homeDir string, args []string) string {
	t.Helper()
	if err := os.MkdirAll(homeDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(homeDir) error = %v", err)
	}

	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	defer func() {
		if err := os.Chdir(oldwd); err != nil {
			t.Fatalf("restore cwd: %v", err)
		}
	}()
	if err := os.Chdir(projectDir); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}

	t.Setenv("AGENTCOM_HOME", homeDir)
	buf := &bytes.Buffer{}
	root := NewRootCmd()
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetIn(bytes.NewReader(nil))
	root.SetArgs(args)
	if err := root.Execute(); err != nil {
		t.Fatalf("root.Execute(%v) error = %v output=%s", args, err, buf.String())
	}
	return buf.String()
}

func decodeDoctorChecks(t *testing.T, output string) []doctorCheckPayload {
	t.Helper()
	var checks []doctorCheckPayload
	if err := json.Unmarshal([]byte(output), &checks); err != nil {
		t.Fatalf("json.Unmarshal() error = %v output=%s", err, output)
	}
	return checks
}

func hasDoctorStatus(checks []doctorCheckPayload, category string, status string) bool {
	for _, check := range checks {
		if check.Category == category && check.Status == status {
			return true
		}
	}
	return false
}

func hasNamedDoctorStatus(checks []doctorCheckPayload, category string, name string, status string) bool {
	for _, check := range checks {
		if check.Category == category && check.Name == name && check.Status == status {
			return true
		}
	}
	return false
}

func hasDoctorFix(checks []doctorCheckPayload) bool {
	for _, check := range checks {
		if check.Fix != "" {
			return true
		}
	}
	return false
}
