package cli

import (
	"errors"
	"fmt"
)

type userError struct {
	What string
	Why  string
	How  string
	err  error
}

func (e *userError) Error() string {
	return fmt.Sprintf("Error: %s\nReason: %s\nHint: %s", e.What, e.Why, e.How)
}

func (e *userError) Unwrap() error {
	return e.err
}

func newUserError(what, why, how string) *userError {
	return &userError{What: what, Why: why, How: how}
}

func wrapUserError(what, why, how string, err error) *userError {
	return &userError{What: what, Why: why, How: how, err: err}
}

func commandError(prefix string, err error) error {
	if err == nil {
		return nil
	}
	var userErr *userError
	if errors.As(err, &userErr) {
		return userErr
	}
	return fmt.Errorf("%s: %w", prefix, err)
}
