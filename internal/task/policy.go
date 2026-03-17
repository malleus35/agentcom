package task

import (
	"fmt"
	"strings"
)

type ReviewPolicyRule struct {
	Priority string `json:"priority" yaml:"priority"`
	Reviewer string `json:"reviewer" yaml:"reviewer"`
}

type ReviewPolicy struct {
	RequireReviewAbove string             `json:"require_review_above,omitempty" yaml:"require_review_above,omitempty"`
	DefaultReviewer    string             `json:"default_reviewer,omitempty" yaml:"default_reviewer,omitempty"`
	Rules              []ReviewPolicyRule `json:"rules,omitempty" yaml:"rules,omitempty"`
}

func (p *ReviewPolicy) Validate() error {
	if p == nil {
		return nil
	}
	if threshold := NormalizePriority(p.RequireReviewAbove); threshold != "" {
		if err := ValidatePriority(threshold); err != nil {
			return fmt.Errorf("validate review policy threshold: %w", err)
		}
	}
	for _, rule := range p.Rules {
		if err := ValidatePriority(rule.Priority); err != nil {
			return fmt.Errorf("validate review policy rule priority: %w", err)
		}
		if strings.TrimSpace(rule.Reviewer) == "" {
			return fmt.Errorf("review policy rule reviewer is required")
		}
	}
	return nil
}

func (p *ReviewPolicy) ResolveReviewer(priority string) string {
	if p == nil {
		return ""
	}
	threshold := NormalizePriority(p.RequireReviewAbove)
	if threshold == "" {
		return ""
	}
	resolvedPriority := NormalizePriority(priority)
	if !PriorityAtLeast(resolvedPriority, threshold) {
		return ""
	}
	for _, rule := range p.Rules {
		if NormalizePriority(rule.Priority) == resolvedPriority {
			return strings.TrimSpace(rule.Reviewer)
		}
	}
	return strings.TrimSpace(p.DefaultReviewer)
}
