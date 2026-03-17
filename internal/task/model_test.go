package task

import (
	"errors"
	"testing"
)

func TestValidatePriority(t *testing.T) {
	tests := []struct {
		name     string
		priority string
		wantErr  bool
	}{
		{name: "low", priority: "low"},
		{name: "medium", priority: "medium"},
		{name: "high upper normalized", priority: "HIGH"},
		{name: "critical spaced normalized", priority: "  critical  "},
		{name: "urgent invalid", priority: "urgent", wantErr: true},
		{name: "asap invalid", priority: "ASAP", wantErr: true},
		{name: "empty invalid", priority: "", wantErr: true},
		{name: "whitespace invalid", priority: "   ", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePriority(tt.priority)
			if tt.wantErr && err == nil {
				t.Fatal("ValidatePriority() error = nil, want error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("ValidatePriority() error = %v", err)
			}
		})
	}
}

func TestNormalizePriority(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{input: "LOW", want: "low"},
		{input: " Medium ", want: "medium"},
		{input: "HiGh", want: "high"},
	}

	for _, tt := range tests {
		if got := NormalizePriority(tt.input); got != tt.want {
			t.Fatalf("NormalizePriority(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestComparePriorityAndAtLeast(t *testing.T) {
	tests := []struct {
		name      string
		a         string
		b         string
		wantCmp   int
		wantAtMin bool
	}{
		{name: "low below medium", a: PriorityLow, b: PriorityMedium, wantCmp: -1, wantAtMin: false},
		{name: "medium equals medium", a: PriorityMedium, b: PriorityMedium, wantCmp: 0, wantAtMin: true},
		{name: "high above medium", a: PriorityHigh, b: PriorityMedium, wantCmp: 1, wantAtMin: true},
		{name: "critical above high", a: PriorityCritical, b: PriorityHigh, wantCmp: 1, wantAtMin: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ComparePriority(tt.a, tt.b); got != tt.wantCmp {
				t.Fatalf("ComparePriority(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.wantCmp)
			}
			if got := PriorityAtLeast(tt.a, tt.b); got != tt.wantAtMin {
				t.Fatalf("PriorityAtLeast(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.wantAtMin)
			}
		})
	}
}

func TestValidateTransition(t *testing.T) {
	tests := []struct {
		name    string
		from    string
		to      string
		wantErr error
	}{
		{name: "pending to assigned", from: StatusPending, to: StatusAssigned},
		{name: "assigned to in_progress", from: StatusAssigned, to: StatusInProgress},
		{name: "blocked to completed", from: StatusBlocked, to: StatusCompleted},
		{name: "completed to pending", from: StatusCompleted, to: StatusPending},
		{name: "completed to cancelled", from: StatusCompleted, to: StatusCancelled},
		{name: "failed to pending", from: StatusFailed, to: StatusPending},
		{name: "failed to cancelled", from: StatusFailed, to: StatusCancelled},
		{name: "cancelled to pending", from: StatusCancelled, to: StatusPending},
		{name: "pending to failed invalid", from: StatusPending, to: StatusFailed, wantErr: ErrInvalidTransition},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTransition(tt.from, tt.to)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("ValidateTransition() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateTransitionWithReviewer(t *testing.T) {
	tests := []struct {
		name     string
		from     string
		to       string
		reviewer string
		wantErr  error
	}{
		{name: "reviewer blocks direct completion", from: StatusInProgress, to: StatusCompleted, reviewer: "user", wantErr: ErrInvalidTransition},
		{name: "no reviewer allows completion", from: StatusInProgress, to: StatusCompleted},
		{name: "reviewer allows blocked to completed", from: StatusBlocked, to: StatusCompleted, reviewer: "user"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTransitionWithReviewer(tt.from, tt.to, tt.reviewer)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("ValidateTransitionWithReviewer() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestIsTerminal(t *testing.T) {
	tests := []struct {
		status string
		want   bool
	}{
		{status: StatusCompleted, want: true},
		{status: StatusFailed, want: true},
		{status: StatusCancelled, want: true},
		{status: StatusPending, want: false},
		{status: StatusAssigned, want: false},
	}

	for _, tt := range tests {
		if got := IsTerminal(tt.status); got != tt.want {
			t.Fatalf("IsTerminal(%q) = %v, want %v", tt.status, got, tt.want)
		}
	}
}
