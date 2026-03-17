package onboard

import (
	"context"
	"errors"
	"testing"
)

type interfacePrompter struct{}

func (interfacePrompter) Run(ctx context.Context, defaults Result) (Result, error) {
	return defaults, nil
}

type interfaceApplier struct{}

func (interfaceApplier) Apply(ctx context.Context, result Result) (ApplyReport, error) {
	return ApplyReport{HomeDir: result.HomeDir, Project: result.Project, Status: "initialized"}, nil
}

func TestPrompterAndApplierInterfaces(t *testing.T) {
	var prompter Prompter = interfacePrompter{}
	var applier Applier = interfaceApplier{}

	result := Result{HomeDir: "/tmp/agentcom", Project: "demo-app", Confirmed: true}
	got, err := prompter.Run(context.Background(), result)
	if err != nil {
		t.Fatalf("Prompter.Run() error = %v", err)
	}
	if got.Project != result.Project {
		t.Fatalf("Prompter.Run().Project = %q, want %q", got.Project, result.Project)
	}
	report, err := applier.Apply(context.Background(), got)
	if err != nil {
		t.Fatalf("Applier.Apply() error = %v", err)
	}
	if report.HomeDir != result.HomeDir {
		t.Fatalf("Applier.Apply().HomeDir = %q, want %q", report.HomeDir, result.HomeDir)
	}
}

func TestErrAbortedIsSentinel(t *testing.T) {
	wrapped := errors.Join(ErrAborted, errors.New("cancelled"))
	if !errors.Is(wrapped, ErrAborted) {
		t.Fatal("errors.Is(wrapped, ErrAborted) = false, want true")
	}
}
