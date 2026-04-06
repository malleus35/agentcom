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

// PH11 T4.1.2 — install-failure report shape contract.
//
// The report is opt-in and consumed by humans pasting JSON into a GitHub issue
// template, so its schema must be stable and free of personal information.
func TestInstallFailureReportShape(t *testing.T) {
	report := buildInstallFailureReport()
	if report.Schema != installFailureReportSchema {
		t.Errorf("Schema = %d, want %d", report.Schema, installFailureReportSchema)
	}
	if report.OS == "" {
		t.Error("OS must be populated from runtime.GOOS")
	}
	if report.Arch == "" {
		t.Error("Arch must be populated from runtime.GOARCH")
	}
	if report.GoVersion == "" {
		t.Error("GoVersion must be populated from runtime.Version()")
	}
	if report.Notes == "" {
		t.Error("Notes must include the issue-template URL hint")
	}

	// Round-trip through JSON to lock the field shape used by triage tooling.
	encoded, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	for _, key := range []string{`"schema"`, `"agentcom_version"`, `"go_version"`, `"os"`, `"arch"`, `"notes"`} {
		if !bytes.Contains(encoded, []byte(key)) {
			t.Errorf("install-failure JSON is missing required key %s", key)
		}
	}

	// Privacy contract: the report must never leak the user's home directory
	// or hostname. Both env vars are set in this test process so we can detect
	// accidental inclusion regardless of the host machine.
	t.Setenv("HOME", "/tmp/agentcom-test-home-should-not-leak")
	t.Setenv("HOSTNAME", "agentcom-test-host-should-not-leak")
	encoded, err = json.Marshal(buildInstallFailureReport())
	if err != nil {
		t.Fatalf("json.Marshal (privacy): %v", err)
	}
	for _, leak := range []string{"agentcom-test-home-should-not-leak", "agentcom-test-host-should-not-leak"} {
		if bytes.Contains(encoded, []byte(leak)) {
			t.Errorf("install-failure report leaked %q", leak)
		}
	}
}

func TestDoctorReportInstallFailureFlag(t *testing.T) {
	root := NewRootCmd()
	root.SetArgs([]string{"doctor", "--report-install-failure"})
	var stdout bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stdout)
	if err := root.Execute(); err != nil {
		t.Fatalf("doctor --report-install-failure error = %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &parsed); err != nil {
		t.Fatalf("output is not JSON: %v\noutput=%s", err, stdout.String())
	}
	if got, ok := parsed["schema"].(float64); !ok || int(got) != installFailureReportSchema {
		t.Errorf("schema field = %v, want %d", parsed["schema"], installFailureReportSchema)
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
