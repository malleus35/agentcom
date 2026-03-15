package onboard

import (
	"context"
	"errors"
	"testing"
)

type stubPrompter struct {
	result Result
	err    error
}

func (s stubPrompter) Run(_ context.Context, _ Result) (Result, error) {
	return s.result, s.err
}

type stubApplier struct {
	report ApplyReport
	err    error
	called bool
}

func (s *stubApplier) Apply(_ context.Context, _ Result) (ApplyReport, error) {
	s.called = true
	return s.report, s.err
}

func TestWizardRun(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		applier := &stubApplier{report: ApplyReport{HomeDir: "/tmp/agentcom", Status: "initialized"}}
		wizard := NewWizard(stubPrompter{result: Result{HomeDir: "/tmp/agentcom", Confirmed: true}}, applier)

		report, err := wizard.Run(context.Background(), Result{})
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if !applier.called {
			t.Fatal("Apply() was not called")
		}
		if report.Status != "initialized" {
			t.Fatalf("report.Status = %q, want initialized", report.Status)
		}
	})

	t.Run("prompter error", func(t *testing.T) {
		applier := &stubApplier{}
		wizard := NewWizard(stubPrompter{err: errors.New("boom")}, applier)
		if _, err := wizard.Run(context.Background(), Result{}); err == nil {
			t.Fatal("Run() error = nil, want error")
		}
		if applier.called {
			t.Fatal("Apply() called unexpectedly")
		}
	})

	t.Run("validation error", func(t *testing.T) {
		applier := &stubApplier{}
		wizard := NewWizard(stubPrompter{result: Result{HomeDir: "relative", Confirmed: true}}, applier)
		if _, err := wizard.Run(context.Background(), Result{}); err == nil {
			t.Fatal("Run() error = nil, want error")
		}
		if applier.called {
			t.Fatal("Apply() called unexpectedly")
		}
	})

	t.Run("applier error", func(t *testing.T) {
		applier := &stubApplier{err: errors.New("apply failed")}
		wizard := NewWizard(stubPrompter{result: Result{HomeDir: "/tmp/agentcom", Confirmed: true}}, applier)
		if _, err := wizard.Run(context.Background(), Result{}); err == nil {
			t.Fatal("Run() error = nil, want error")
		}
		if !applier.called {
			t.Fatal("Apply() was not called")
		}
	})
}
