package task

import "testing"

func TestReviewPolicyValidate(t *testing.T) {
	tests := []struct {
		name    string
		policy  *ReviewPolicy
		wantErr bool
	}{
		{name: "nil policy", policy: nil},
		{name: "empty policy", policy: &ReviewPolicy{}},
		{name: "valid policy", policy: &ReviewPolicy{RequireReviewAbove: PriorityHigh, DefaultReviewer: "user", Rules: []ReviewPolicyRule{{Priority: PriorityCritical, Reviewer: "user"}}}},
		{name: "invalid threshold", policy: &ReviewPolicy{RequireReviewAbove: "urgent"}, wantErr: true},
		{name: "invalid rule priority", policy: &ReviewPolicy{Rules: []ReviewPolicyRule{{Priority: "urgent", Reviewer: "user"}}}, wantErr: true},
		{name: "missing rule reviewer", policy: &ReviewPolicy{Rules: []ReviewPolicyRule{{Priority: PriorityHigh, Reviewer: ""}}}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.policy.Validate()
			if tt.wantErr && err == nil {
				t.Fatal("Validate() error = nil, want error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("Validate() error = %v", err)
			}
		})
	}
}

func TestReviewPolicyResolveReviewer(t *testing.T) {
	policy := &ReviewPolicy{
		RequireReviewAbove: PriorityHigh,
		DefaultReviewer:    "user",
		Rules:              []ReviewPolicyRule{{Priority: PriorityCritical, Reviewer: "user"}, {Priority: PriorityHigh, Reviewer: "review"}},
	}

	tests := []struct {
		priority string
		want     string
	}{
		{priority: PriorityLow, want: ""},
		{priority: PriorityMedium, want: ""},
		{priority: PriorityHigh, want: "review"},
		{priority: PriorityCritical, want: "user"},
	}

	for _, tt := range tests {
		if got := policy.ResolveReviewer(tt.priority); got != tt.want {
			t.Fatalf("ResolveReviewer(%q) = %q, want %q", tt.priority, got, tt.want)
		}
	}

	defaultPolicy := &ReviewPolicy{RequireReviewAbove: PriorityHigh, DefaultReviewer: "user"}
	if got := defaultPolicy.ResolveReviewer(PriorityCritical); got != "user" {
		t.Fatalf("ResolveReviewer(default) = %q, want user", got)
	}
	if got := (*ReviewPolicy)(nil).ResolveReviewer(PriorityHigh); got != "" {
		t.Fatalf("nil ResolveReviewer() = %q, want empty string", got)
	}
}
