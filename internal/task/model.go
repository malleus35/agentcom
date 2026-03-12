package task

import "errors"

const (
	StatusPending    = "pending"
	StatusAssigned   = "assigned"
	StatusInProgress = "in_progress"
	StatusBlocked    = "blocked"
	StatusCompleted  = "completed"
	StatusFailed     = "failed"
	StatusCancelled  = "cancelled"
)

var ErrInvalidTransition = errors.New("invalid status transition")

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
		StatusCancelled:  {},
	},
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

// IsTerminal reports whether a status is terminal.
func IsTerminal(status string) bool {
	switch status {
	case StatusCompleted, StatusFailed, StatusCancelled:
		return true
	default:
		return false
	}
}
