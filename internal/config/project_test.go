package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteProjectConfig(t *testing.T) {
	dir := t.TempDir()

	path, err := WriteProjectConfig(dir, "my-project")
	if err != nil {
		t.Fatalf("WriteProjectConfig() error = %v", err)
	}
	if path != filepath.Join(dir, ProjectConfigFileName) {
		t.Fatalf("path = %q, want %q", path, filepath.Join(dir, ProjectConfigFileName))
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !strings.Contains(string(data), `"project": "my-project"`) {
		t.Fatalf("config file = %q, want project entry", string(data))
	}

	if _, err := WriteProjectConfig(dir, "my-project"); err == nil {
		t.Fatal("second WriteProjectConfig() error = nil, want error")
	}
}

func TestLoadProjectConfig(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "a", "b", "c")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	path, err := WriteProjectConfig(root, "sample_app")
	if err != nil {
		t.Fatalf("WriteProjectConfig() error = %v", err)
	}

	cfg, loadedPath, err := LoadProjectConfig(nested)
	if err != nil {
		t.Fatalf("LoadProjectConfig() error = %v", err)
	}
	if cfg.Project != "sample_app" {
		t.Fatalf("Project = %q, want sample_app", cfg.Project)
	}
	if loadedPath != path {
		t.Fatalf("loadedPath = %q, want %q", loadedPath, path)
	}
}

func TestLoadProjectConfigHandlesMissingAndInvalidJSON(t *testing.T) {
	dir := t.TempDir()

	cfg, loadedPath, err := LoadProjectConfig(dir)
	if err != nil {
		t.Fatalf("LoadProjectConfig(missing) error = %v", err)
	}
	if cfg.Project != "" || loadedPath != "" {
		t.Fatalf("LoadProjectConfig(missing) = (%#v, %q), want empty values", cfg, loadedPath)
	}

	badPath := filepath.Join(dir, ProjectConfigFileName)
	if err := os.WriteFile(badPath, []byte(`{bad json}`), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if _, _, err := LoadProjectConfig(dir); err == nil {
		t.Fatal("LoadProjectConfig(invalid json) error = nil, want error")
	}
}

func TestResolveProject(t *testing.T) {
	dir := t.TempDir()
	if _, err := WriteProjectConfig(dir, "from-config"); err != nil {
		t.Fatalf("WriteProjectConfig() error = %v", err)
	}

	project, err := ResolveProject("from-flag", dir)
	if err != nil {
		t.Fatalf("ResolveProject(flag) error = %v", err)
	}
	if project != "from-flag" {
		t.Fatalf("ResolveProject(flag) = %q, want from-flag", project)
	}

	project, err = ResolveProject("", dir)
	if err != nil {
		t.Fatalf("ResolveProject(config) error = %v", err)
	}
	if project != "from-config" {
		t.Fatalf("ResolveProject(config) = %q, want from-config", project)
	}
}

func TestValidateProjectName(t *testing.T) {
	tests := []struct {
		name    string
		project string
		wantErr bool
	}{
		{name: "empty allowed", project: ""},
		{name: "valid hyphen", project: "my-app"},
		{name: "valid underscore", project: "my_app_2"},
		{name: "uppercase rejected", project: "MyApp", wantErr: true},
		{name: "spaces rejected", project: "my app", wantErr: true},
		{name: "too long", project: strings.Repeat("a", maxProjectNameLength+1), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateProjectName(tt.project)
			if tt.wantErr && err == nil {
				t.Fatal("ValidateProjectName() error = nil, want error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("ValidateProjectName() error = %v", err)
			}
		})
	}
}
