package onboard

import (
	"context"
	"fmt"
)

// Wizard orchestrates prompt collection and result application.
type Wizard struct {
	prompter Prompter
	applier  Applier
}

// NewWizard creates a new onboarding wizard.
func NewWizard(prompter Prompter, applier Applier) *Wizard {
	return &Wizard{prompter: prompter, applier: applier}
}

// Run executes the onboarding flow and applies the result.
func (w *Wizard) Run(ctx context.Context, defaults Result) (ApplyReport, error) {
	result, err := w.prompter.Run(ctx, defaults)
	if err != nil {
		return ApplyReport{}, fmt.Errorf("onboard.Wizard.Run: %w", err)
	}
	if err := result.Validate(); err != nil {
		return ApplyReport{}, fmt.Errorf("onboard.Wizard.Run: %w", err)
	}
	report, err := w.applier.Apply(ctx, result)
	if err != nil {
		return ApplyReport{}, fmt.Errorf("onboard.Wizard.Run: %w", err)
	}
	return report, nil
}
