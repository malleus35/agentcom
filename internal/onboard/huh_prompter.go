package onboard

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"charm.land/huh/v2"
)

// HuhPrompter implements the onboarding wizard using huh forms.
type HuhPrompter struct {
	accessible bool
	input      io.Reader
	output     io.Writer
}

// NewHuhPrompter creates a new huh-backed onboarding prompter.
func NewHuhPrompter(accessible bool, input io.Reader, output io.Writer) *HuhPrompter {
	return &HuhPrompter{accessible: accessible, input: input, output: output}
}

// Run displays the onboarding wizard and returns the collected answers.
func (p *HuhPrompter) Run(ctx context.Context, defaults Result) (Result, error) {
	homeDir := defaults.HomeDir
	templateChoice := defaults.Template
	if templateChoice == "" {
		templateChoice = "none"
	}
	writeAgentsMD := defaults.WriteAgentsMD
	confirmed := defaults.Confirmed

	summary := func() string {
		templateLabel := templateChoice
		if templateLabel == "none" {
			templateLabel = "none"
		}
		agentsMD := "no"
		if writeAgentsMD {
			agentsMD = "yes"
		}
		return fmt.Sprintf("home: %s\ntemplate: %s\nwrite AGENTS.md: %s", homeDir, templateLabel, agentsMD)
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().Title("agentcom setup").Description("Prepare your local agentcom home and optional project scaffold."),
			huh.NewInput().
				Title("Agentcom home directory").
				Placeholder(filepath.Clean(homeDir)).
				Value(&homeDir).
				Validate(func(value string) error {
					if strings.TrimSpace(value) == "" {
						return errors.New("home directory is required")
					}
					if !filepath.IsAbs(value) {
						return errors.New("home directory must be an absolute path")
					}
					return nil
				}),
		).Title("Step 1: Environment"),
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Project template").
				Options(
					huh.NewOption("None", "none"),
					huh.NewOption("Company", "company"),
					huh.NewOption("Oh-My-OpenCode", "oh-my-opencode"),
				).
				Value(&templateChoice),
			huh.NewConfirm().
				Title("Generate project AGENTS.md in the current directory?").
				Affirmative("Yes").
				Negative("No").
				Value(&writeAgentsMD),
		).Title("Step 2: Project scaffold"),
		huh.NewGroup(
			huh.NewNote().Title("Review selections").DescriptionFunc(summary, []any{&homeDir, &templateChoice, &writeAgentsMD}),
			huh.NewConfirm().
				Title("Apply these settings?").
				Affirmative("Apply").
				Negative("Cancel").
				Value(&confirmed),
		).Title("Step 3: Confirm"),
	).
		WithAccessible(p.accessible).
		WithInput(p.input).
		WithOutput(p.output)

	if err := form.RunWithContext(ctx); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return Result{}, ErrAborted
		}
		return Result{}, fmt.Errorf("onboard.HuhPrompter.Run: %w", err)
	}

	result := Result{
		HomeDir:       homeDir,
		Template:      templateChoice,
		WriteAgentsMD: writeAgentsMD,
		Confirmed:     confirmed,
	}
	if result.Template == "none" {
		result.Template = ""
	}
	return result, nil
}
