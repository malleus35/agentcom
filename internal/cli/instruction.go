package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type instructionFileDefinition struct {
	AgentID            string
	Aliases            []string
	FileName           string
	RelativePath       string
	Format             string
	SupportsMemory     bool
	MemoryFileName     string
	MemoryRelativePath string
}

var instructionFileDefinitions = []instructionFileDefinition{
	{AgentID: "claude", Aliases: []string{"claude-code"}, FileName: "CLAUDE.md", RelativePath: "CLAUDE.md", Format: "markdown"},
	{AgentID: "codex", FileName: "AGENTS.md", RelativePath: "AGENTS.md", Format: "markdown", SupportsMemory: true, MemoryFileName: "MEMORY.md", MemoryRelativePath: filepath.Join(".agents", "MEMORY.md")},
	{AgentID: "gemini", Aliases: []string{"gemini-cli"}, FileName: "GEMINI.md", RelativePath: "GEMINI.md", Format: "markdown"},
	{AgentID: "cursor", FileName: "agentcom.mdc", RelativePath: filepath.Join(".cursor", "rules", "agentcom.mdc"), Format: "mdc"},
	{AgentID: "github-copilot", FileName: "copilot-instructions.md", RelativePath: filepath.Join(".github", "copilot-instructions.md"), Format: "markdown"},
	{AgentID: "windsurf", FileName: ".windsurfrules", RelativePath: ".windsurfrules", Format: "markdown"},
	{AgentID: "cline", FileName: ".clinerules", RelativePath: ".clinerules", Format: "markdown"},
	{AgentID: "roo-code", FileName: ".roorules", RelativePath: ".roorules", Format: "markdown"},
	{AgentID: "amazon-q", FileName: "agentcom.md", RelativePath: filepath.Join(".amazonq", "rules", "agentcom.md"), Format: "markdown"},
	{AgentID: "augment-code", FileName: "agentcom.md", RelativePath: filepath.Join(".augment", "rules", "agentcom.md"), Format: "markdown"},
	{AgentID: "continue", FileName: "agentcom.md", RelativePath: filepath.Join(".continue", "rules", "agentcom.md"), Format: "markdown"},
	{AgentID: "kilo-code", FileName: "agentcom.md", RelativePath: filepath.Join(".kilocode", "rules", "agentcom.md"), Format: "markdown"},
	{AgentID: "trae", FileName: "project_rules.md", RelativePath: filepath.Join(".trae", "project_rules.md"), Format: "markdown"},
	{AgentID: "goose", FileName: ".goosehints", RelativePath: ".goosehints", Format: "markdown"},
	{AgentID: "opencode", FileName: "AGENTS.md", RelativePath: "AGENTS.md", Format: "markdown"},
	{AgentID: "amp", FileName: "AGENTS.md", RelativePath: "AGENTS.md", Format: "markdown"},
	{AgentID: "devin", FileName: "AGENTS.md", RelativePath: "AGENTS.md", Format: "markdown"},
	{AgentID: "aider", FileName: "CONVENTIONS.md", RelativePath: "CONVENTIONS.md", Format: "markdown"},
	{AgentID: "universal", FileName: "AGENTS.md", RelativePath: "AGENTS.md", Format: "markdown", SupportsMemory: true, MemoryFileName: "MEMORY.md", MemoryRelativePath: filepath.Join(".agentcom", "MEMORY.md")},
}

func resolveInstructionAgents(raw string) ([]string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil, nil
	}
	if value == "all" {
		agents := make([]string, 0, len(instructionFileDefinitions))
		for _, definition := range instructionFileDefinitions {
			if definition.AgentID == "universal" {
				continue
			}
			agents = append(agents, definition.AgentID)
		}
		return agents, nil
	}

	parts := strings.Split(value, ",")
	agents := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		agent := strings.TrimSpace(part)
		if agent == "" {
			continue
		}
		definition, ok := findInstructionDefinition(agent)
		if !ok {
			return nil, fmt.Errorf("invalid agent %q", agent)
		}
		if _, ok := seen[definition.AgentID]; ok {
			continue
		}
		seen[definition.AgentID] = struct{}{}
		agents = append(agents, definition.AgentID)
	}

	if len(agents) == 0 {
		return nil, nil
	}
	return agents, nil
}

func findInstructionDefinition(agentID string) (instructionFileDefinition, bool) {
	for _, definition := range instructionFileDefinitions {
		if definition.AgentID == agentID {
			return definition, true
		}
		for _, alias := range definition.Aliases {
			if alias == agentID {
				return definition, true
			}
		}
	}

	return instructionFileDefinition{}, false
}

func renderInstructionContent(agentID string, projectName string) (string, error) {
	definition, ok := findInstructionDefinition(agentID)
	if !ok {
		return "", fmt.Errorf("unsupported instruction agent %q", agentID)
	}

	workflow := instructionWorkflowBody(projectName)
	common := strings.TrimSpace(fmt.Sprintf(`# %s

## agentcom Workflow

%s

## Recommended Conventions

- Use stable agent names per worktree or terminal session.
- Keep one registered process per agent name.
- Prefer JSON payloads for structured messages between agents.
- Deregister agents cleanly on shutdown, or let `+"`register`"+` auto-clean up on signal.
`, definition.FileName, workflow))

	switch definition.Format {
	case "mdc":
		return fmt.Sprintf("---\ndescription: agentcom workflow instructions\nalwaysApply: true\n---\n\n%s\n", common), nil
	case "markdown":
		return common + "\n", nil
	default:
		return "", fmt.Errorf("unsupported instruction format %q", definition.Format)
	}
}

func renderMemoryContent(agentID string) (string, error) {
	definition, ok := findInstructionDefinition(agentID)
	if !ok {
		return "", fmt.Errorf("unsupported instruction agent %q", agentID)
	}
	if !definition.SupportsMemory {
		return "", fmt.Errorf("agent %q does not support memory files", agentID)
	}

	return fmt.Sprintf(`# %s

## Current State

- Track the current phase, active branch, and the next concrete action.

## Completed Work

- Record finished tasks with enough detail to resume safely in a later session.

## Decisions

- Capture each important decision with the reason it was made.

## Open Issues

- Keep unresolved bugs, blockers, or follow-up questions here.

## Next Session

- Leave the exact starting point for the next agent session.
`, definition.MemoryFileName), nil
}

func writeAgentInstructions(projectDir string, agentIDs []string) ([]string, error) {
	projectName := filepath.Base(projectDir)
	generated := make([]string, 0, len(agentIDs))
	seenAgents := make(map[string]struct{}, len(agentIDs))
	seenPaths := make(map[string]struct{}, len(agentIDs))

	for _, agentID := range agentIDs {
		definition, ok := findInstructionDefinition(agentID)
		if !ok {
			return generated, fmt.Errorf("unsupported instruction agent %q", agentID)
		}
		if _, ok := seenAgents[definition.AgentID]; ok {
			continue
		}
		seenAgents[definition.AgentID] = struct{}{}

		content, err := renderInstructionContent(definition.AgentID, projectName)
		if err != nil {
			return generated, fmt.Errorf("render instruction content for %s: %w", definition.AgentID, err)
		}

		path := filepath.Join(projectDir, definition.RelativePath)
		if _, ok := seenPaths[path]; ok {
			continue
		}
		if err := writeInstructionFile(path, content); err != nil {
			return generated, fmt.Errorf("write instruction file for %s: %w", definition.AgentID, err)
		}
		seenPaths[path] = struct{}{}
		generated = append(generated, path)
	}

	sort.Strings(generated)
	return generated, nil
}

func writeAgentMemoryFiles(projectDir string, agentIDs []string) ([]string, error) {
	generated := make([]string, 0, len(agentIDs))
	seen := make(map[string]struct{}, len(agentIDs))

	for _, agentID := range agentIDs {
		definition, ok := findInstructionDefinition(agentID)
		if !ok {
			return generated, fmt.Errorf("unsupported instruction agent %q", agentID)
		}
		if !definition.SupportsMemory {
			continue
		}
		if _, ok := seen[definition.MemoryRelativePath]; ok {
			continue
		}
		seen[definition.MemoryRelativePath] = struct{}{}

		content, err := renderMemoryContent(definition.AgentID)
		if err != nil {
			return generated, fmt.Errorf("render memory content for %s: %w", definition.AgentID, err)
		}

		path := filepath.Join(projectDir, definition.MemoryRelativePath)
		if err := writeInstructionFile(path, content); err != nil {
			return generated, fmt.Errorf("write memory file for %s: %w", definition.AgentID, err)
		}
		generated = append(generated, path)
	}

	sort.Strings(generated)
	return generated, nil
}

func writeProjectAgentsMD(path string) error {
	projectDir := filepath.Dir(path)
	generated, err := writeAgentInstructions(projectDir, []string{"codex"})
	if err != nil {
		return err
	}
	if len(generated) != 1 || generated[0] != path {
		return fmt.Errorf("unexpected AGENTS.md path %q", strings.Join(generated, ", "))
	}
	return nil
}

func writeInstructionFile(path string, content string) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("instruction file already exists: %s", path)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat instruction file: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir instruction dir: %w", err)
	}

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write instruction file: %w", err)
	}

	return nil
}

func instructionWorkflowBody(projectName string) string {
	if strings.TrimSpace(projectName) == "" {
		projectName = "this project"
	}

	return strings.TrimSpace(fmt.Sprintf(`- Work inside %q and keep instructions aligned with the current repository state.
- Run `+"`agentcom init`"+` once per machine to create the local SQLite database and socket directories.
- Register each long-running agent session with `+"`agentcom register --name <name> --type <type>`"+`.
- Send direct messages with `+"`agentcom send --from <sender> <target> <message-or-json>`"+`.
- Broadcast updates with `+"`agentcom broadcast --from <sender> <message-or-json>`"+`.
- Create and delegate tasks with `+"`agentcom task create`"+` and `+"`agentcom task delegate`"+`.
- Check inbox and system status with `+"`agentcom inbox`"+` and `+"`agentcom status`"+`.
- Start MCP mode with `+"`agentcom mcp-server`"+` for tool-based integrations.`, projectName))
}
