package onboard

import (
	"context"
	"errors"
)

// ErrAborted is returned when the user cancels the onboarding flow.
var ErrAborted = errors.New("onboard: aborted")

// Prompter collects onboarding answers from the user.
type Prompter interface {
	Run(ctx context.Context, defaults Result) (Result, error)
}

// Applier materializes an onboarding result into files and local state.
type Applier interface {
	Apply(ctx context.Context, result Result) (ApplyReport, error)
}
