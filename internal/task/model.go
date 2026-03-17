package task

import (
	"errors"
	"fmt"
	"strings"
)

const (
	PriorityLow      = "low"
	PriorityMedium   = "medium"
	PriorityHigh     = "high"
	PriorityCritical = "critical"

	StatusPending    = "pending"
	StatusAssigned   = "assigned"
	StatusInProgress = "in_progress"
	StatusBlocked    = "blocked"
	StatusCompleted  = "completed"
	StatusFailed     = "failed"
	StatusCancelled  = "cancelled"
)

var ErrInvalidTransition = errors.New("invalid status transition")

var ValidPriorities = map[string]struct{}{
	PriorityLow:      {},
	PriorityMedium:   {},
	PriorityHigh:     {},
	PriorityCritical: {},
}

var priorityOrder = map[string]int{
	PriorityLow:      0,
	PriorityMedium:   1,
	PriorityHigh:     2,
	PriorityCritical: 3,
}

var validTransitions = map[string]map[string]struct{}{
	StatusPending: {
		StatusAssigned:   {},
		StatusInProgress: {},
		StatusCancelled:  {},
	},
	StatusAssigned: {
		StatusInProgress: {},
		StatusBlocked:    {},
		StatusPending:    {},
		StatusCancelled:  {},
	},
	StatusInProgress: {
		StatusCompleted: {},
		StatusFailed:    {},
		StatusBlocked:   {},
	},
	StatusBlocked: {
		StatusInProgress: {},
		StatusPending:    {},
		StatusCompleted:  {},
		StatusCancelled:  {},
	},
}

func NormalizePriority(priority string) string {
	return strings.ToLower(strings.TrimSpace(priority))
}

func ValidatePriority(priority string) error {
	normalized := NormalizePriority(priority)
	if normalized == "" {
		return fmt.Errorf("priority is required")
	}
	if _, ok := ValidPriorities[normalized]; !ok {
		return fmt.Errorf("invalid priority %q", priority)
	}
	return nil
}

func ComparePriority(a, b string) int {
	aOrder := priorityOrder[NormalizePriority(a)]
	bOrder := priorityOrder[NormalizePriority(b)]
	switch {
	case aOrder > bOrder:
		return 1
	case aOrder < bOrder:
		return -1
	default:
		return 0
	}
}

func PriorityAtLeast(priority string, threshold string) bool {
	return ComparePriority(priority, threshold) >= 0
}

// ValidateTransition validates whether a state transition is allowed.
func ValidateTransition(from, to string) error {
	next, ok := validTransitions[from]
	if !ok {
		return ErrInvalidTransition
	}

	if _, ok := next[to]; !ok {
		return ErrInvalidTransition
	}

	return nil
}

func ValidateTransitionWithReviewer(from, to, reviewer string) error {
	if strings.TrimSpace(reviewer) != "" && from == StatusInProgress && to == StatusCompleted {
		return ErrInvalidTransition
	}
	return ValidateTransition(from, to)
}

// IsTerminal reports whether a status is terminal.
func IsTerminal(status string) bool {
	switch status {
	case StatusCompleted, StatusFailed, StatusCancelled:
		return true
	default:
		return false
	}
}
