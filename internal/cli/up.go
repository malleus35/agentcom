package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/malleus35/agentcom/internal/config"
	"github.com/malleus35/agentcom/internal/db"
	"github.com/spf13/cobra"
)

const upSupervisorCommandName = "__up-supervisor"

const upSupervisorHealthCheckInterval = 5 * time.Second

type templateManifest struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Reference   string                 `json:"reference"`
	CommonTitle string                 `json:"common_title"`
	CommonBody  string                 `json:"common_body"`
	Roles       []templateManifestRole `json:"roles"`
}

type templateManifestRole struct {
	Name      string `json:"name"`
	AgentName string `json:"agent_name"`
	AgentType string `json:"agent_type"`
}

func newUpCmd() *cobra.Command {
	var templateName string
	var onlyRaw string
	var force bool

	cmd := &cobra.Command{
		Use:   "up",
		Short: "Start all agents from the active template",
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("cli.newUpCmd: getwd: %w", err)
			}

			projectCfg, configPath, err := ensureProjectConfigForUp(cmd, cwd)
			if err != nil {
				return fmt.Errorf("cli.newUpCmd: ensure project config: %w", err)
			}
			projectDir := filepath.Dir(configPath)

			if strings.TrimSpace(templateName) != "" {
				projectCfg.Template.Active = strings.TrimSpace(templateName)
				if _, err := config.SaveProjectConfig(projectDir, projectCfg); err != nil {
					return fmt.Errorf("cli.newUpCmd: save project config: %w", err)
				}
			}

			activeTemplate := strings.TrimSpace(projectCfg.Template.Active)
			if activeTemplate == "" {
				return fmt.Errorf("cli.newUpCmd: no active template configured in %s", configPath)
			}

			manifest, err := loadTemplateManifest(projectDir, activeTemplate)
			if err != nil {
				return fmt.Errorf("cli.newUpCmd: load template manifest: %w", err)
			}

			roles, err := filterTemplateRoles(manifest.Roles, onlyRaw)
			if err != nil {
				return fmt.Errorf("cli.newUpCmd: filter roles: %w", err)
			}

			if err := handleExistingRuntimeState(projectDir, force); err != nil {
				return fmt.Errorf("cli.newUpCmd: %w", err)
			}

			startedState, err := startDetachedSupervisor(cmd.Context(), projectDir, projectCfg.Project, activeTemplate, roles)
			if err != nil {
				return fmt.Errorf("cli.newUpCmd: start detached supervisor: %w", err)
			}

			if jsonOutput {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(map[string]any{
					"status":         "started",
					"project":        projectCfg.Project,
					"template":       startedState.Template,
					"supervisor_pid": startedState.SupervisorPID,
					"agents":         startedState.Agents,
					"runtime_state":  upRuntimeStatePath(projectDir),
				})
			}

			_, err = fmt.Fprintf(cmd.OutOrStdout(), "started template %s with %d agent(s); supervisor pid=%d\n", startedState.Template, len(startedState.Agents), startedState.SupervisorPID)
			return err
		},
	}

	cmd.Flags().StringVar(&templateName, "template", "", "Template to start (defaults to .agentcom.json template.active)")
	cmd.Flags().StringVar(&onlyRaw, "only", "", "Comma-separated role names to start")
	cmd.Flags().BoolVar(&force, "force", false, "Stop any existing run state before starting")
	return cmd
}

func newDownCmd() *cobra.Command {
	var onlyRaw string
	var force bool
	var timeoutSeconds int

	cmd := &cobra.Command{
		Use:   "down",
		Short: "Stop agents started by agentcom up",
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("cli.newDownCmd: getwd: %w", err)
			}

			projectDir := cwd
			if _, configPath, cfgErr := config.LoadProjectConfig(cwd); cfgErr == nil && configPath != "" {
				projectDir = filepath.Dir(configPath)
			}

			state, _, err := loadUpRuntimeState(projectDir)
			if err != nil {
				return fmt.Errorf("cli.newDownCmd: load runtime state: %w", err)
			}
			if state.SupervisorPID == 0 {
				return fmt.Errorf("cli.newDownCmd: no running up session found")
			}

			targetRoles, err := parseOnlyRoles(onlyRaw)
			if err != nil {
				return fmt.Errorf("cli.newDownCmd: parse only roles: %w", err)
			}
			timeout := time.Duration(timeoutSeconds) * time.Second

			stopped, remaining, err := stopRuntimeState(projectDir, state, targetRoles, force, timeout)
			if err != nil {
				return fmt.Errorf("cli.newDownCmd: stop runtime state: %w", err)
			}

			if jsonOutput {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(map[string]any{
					"status":          "stopped",
					"stopped_roles":   stopped,
					"remaining_roles": remaining,
				})
			}

			_, err = fmt.Fprintf(cmd.OutOrStdout(), "stopped roles: %s\n", strings.Join(stopped, ", "))
			return err
		},
	}

	cmd.Flags().StringVar(&onlyRaw, "only", "", "Comma-separated role names to stop")
	cmd.Flags().BoolVar(&force, "force", false, "Force kill running processes")
	cmd.Flags().IntVar(&timeoutSeconds, "timeout", 10, "Graceful shutdown timeout in seconds")
	return cmd
}

func newUpSupervisorCmd() *cobra.Command {
	var projectDir string
	var templateName string
	var rolesRaw string

	cmd := &cobra.Command{
		Use:    upSupervisorCommandName,
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			manifest, err := loadTemplateManifest(projectDir, templateName)
			if err != nil {
				return fmt.Errorf("cli.newUpSupervisorCmd: load template manifest: %w", err)
			}
			roles, err := filterTemplateRoles(manifest.Roles, rolesRaw)
			if err != nil {
				return fmt.Errorf("cli.newUpSupervisorCmd: filter roles: %w", err)
			}
			return runUpSupervisor(ctx, projectDir, currentProjectFilter(), templateName, roles)
		},
	}

	cmd.Flags().StringVar(&projectDir, "project-dir", "", "Project directory")
	cmd.Flags().StringVar(&templateName, "template", "", "Template name")
	cmd.Flags().StringVar(&rolesRaw, "roles", "", "Comma-separated role names")
	_ = cmd.MarkFlagRequired("project-dir")
	_ = cmd.MarkFlagRequired("template")
	return cmd
}

func ensureProjectConfigForUp(cmd *cobra.Command, cwd string) (config.ProjectConfig, string, error) {
	projectCfg, configPath, err := config.LoadProjectConfig(cwd)
	if err != nil {
		return config.ProjectConfig{}, "", fmt.Errorf("cli.ensureProjectConfigForUp: %w", err)
	}
	if configPath != "" {
		return projectCfg, configPath, nil
	}
	if !isInteractiveInput(cmd.InOrStdin()) {
		return config.ProjectConfig{}, "", fmt.Errorf("cli.ensureProjectConfigForUp: project is not initialized; run `agentcom init` first")
	}
	if err := runInteractiveInit(cwd); err != nil {
		return config.ProjectConfig{}, "", fmt.Errorf("cli.ensureProjectConfigForUp: auto init failed: %w", err)
	}
	projectCfg, configPath, err = config.LoadProjectConfig(cwd)
	if err != nil {
		return config.ProjectConfig{}, "", fmt.Errorf("cli.ensureProjectConfigForUp: reload project config: %w", err)
	}
	if configPath == "" {
		return config.ProjectConfig{}, "", fmt.Errorf("cli.ensureProjectConfigForUp: init completed without creating %s", config.ProjectConfigFileName)
	}
	return projectCfg, configPath, nil
}

func runInteractiveInit(cwd string) error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cli.runInteractiveInit: executable: %w", err)
	}
	cmd := exec.Command(exePath, "init")
	cmd.Dir = cwd
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("cli.runInteractiveInit: %w", err)
	}
	return nil
}

func loadTemplateManifest(projectDir string, templateName string) (templateManifest, error) {
	path := filepath.Join(projectDir, ".agentcom", "templates", templateName, "template.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return templateManifest{}, fmt.Errorf("cli.loadTemplateManifest: read %s: %w", path, err)
	}
	var manifest templateManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return templateManifest{}, fmt.Errorf("cli.loadTemplateManifest: decode %s: %w", path, err)
	}
	return manifest, nil
}

func filterTemplateRoles(roles []templateManifestRole, onlyRaw string) ([]templateManifestRole, error) {
	targets, err := parseOnlyRoles(onlyRaw)
	if err != nil {
		return nil, err
	}
	if len(targets) == 0 {
		cloned := append([]templateManifestRole(nil), roles...)
		sort.Slice(cloned, func(i, j int) bool { return cloned[i].Name < cloned[j].Name })
		return cloned, nil
	}

	byName := make(map[string]templateManifestRole, len(roles))
	for _, role := range roles {
		byName[role.Name] = role
	}
	filtered := make([]templateManifestRole, 0, len(targets))
	for _, target := range targets {
		role, ok := byName[target]
		if !ok {
			return nil, fmt.Errorf("unknown role %q", target)
		}
		filtered = append(filtered, role)
	}
	return filtered, nil
}

func parseOnlyRoles(raw string) ([]string, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	parts := strings.Split(raw, ",")
	seen := make(map[string]struct{}, len(parts))
	roles := make([]string, 0, len(parts))
	for _, part := range parts {
		role := strings.TrimSpace(part)
		if role == "" {
			continue
		}
		if _, ok := seen[role]; ok {
			continue
		}
		seen[role] = struct{}{}
		roles = append(roles, role)
	}
	if len(roles) == 0 {
		return nil, fmt.Errorf("empty role selection")
	}
	return roles, nil
}

func handleExistingRuntimeState(projectDir string, force bool) error {
	state, _, err := loadUpRuntimeState(projectDir)
	if err != nil {
		return err
	}
	if state.SupervisorPID == 0 {
		return nil
	}
	if !processAliveCheck(state.SupervisorPID) {
		if err := cleanupStaleRuntimeState(projectDir, state); err != nil {
			return err
		}
		return nil
	}
	if !force {
		return fmt.Errorf("an up session is already running")
	}
	_, _, err = stopRuntimeState(projectDir, state, nil, true, 5*time.Second)
	return err
}

func startDetachedSupervisor(ctx context.Context, projectDir, projectName, templateName string, roles []templateManifestRole) (upRuntimeState, error) {
	runDir := filepath.Dir(upRuntimeStatePath(projectDir))
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		return upRuntimeState{}, fmt.Errorf("cli.startDetachedSupervisor: mkdir run dir: %w", err)
	}
	logPath := filepath.Join(runDir, "up.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return upRuntimeState{}, fmt.Errorf("cli.startDetachedSupervisor: open log: %w", err)
	}
	defer logFile.Close()

	exePath, err := os.Executable()
	if err != nil {
		return upRuntimeState{}, fmt.Errorf("cli.startDetachedSupervisor: executable: %w", err)
	}

	args := []string{}
	if projectName != "" {
		args = append(args, "--project", projectName)
	}
	args = append(args, upSupervisorCommandName, "--project-dir", projectDir, "--template", templateName)
	if len(roles) > 0 {
		roleNames := make([]string, 0, len(roles))
		for _, role := range roles {
			roleNames = append(roleNames, role.Name)
		}
		args = append(args, "--roles", strings.Join(roleNames, ","))
	}

	cmd := exec.Command(exePath, args...)
	cmd.Dir = projectDir
	cmd.Env = os.Environ()
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := configureDetachedProcess(cmd); err != nil {
		return upRuntimeState{}, fmt.Errorf("cli.startDetachedSupervisor: configure detached process: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return upRuntimeState{}, fmt.Errorf("cli.startDetachedSupervisor: start: %w", err)
	}

	state, err := waitForRuntimeState(ctx, projectDir, cmd, logPath)
	if err != nil {
		return upRuntimeState{}, err
	}
	return state, nil
}

func waitForRuntimeState(ctx context.Context, projectDir string, cmd *exec.Cmd, logPath string) (upRuntimeState, error) {
	waitCh := make(chan error, 1)
	go func() {
		waitCh <- cmd.Wait()
	}()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	timeout := time.NewTimer(5 * time.Second)
	defer timeout.Stop()

	for {
		select {
		case <-ctx.Done():
			return upRuntimeState{}, fmt.Errorf("cli.waitForRuntimeState: %w", ctx.Err())
		case err := <-waitCh:
			logData, _ := os.ReadFile(logPath)
			return upRuntimeState{}, fmt.Errorf("cli.waitForRuntimeState: supervisor exited early: %w: %s", err, strings.TrimSpace(string(logData)))
		case <-ticker.C:
			state, _, err := loadUpRuntimeState(projectDir)
			if err == nil && state.SupervisorPID != 0 && len(state.Agents) > 0 {
				return state, nil
			}
		case <-timeout.C:
			logData, _ := os.ReadFile(logPath)
			return upRuntimeState{}, fmt.Errorf("cli.waitForRuntimeState: timed out waiting for runtime state: %s", strings.TrimSpace(string(logData)))
		}
	}
}

func runUpSupervisor(ctx context.Context, projectDir, projectName, templateName string, roles []templateManifestRole) error {
	children := make([]*exec.Cmd, 0, len(roles))
	state := upRuntimeState{
		Project:       projectName,
		ProjectDir:    projectDir,
		Template:      templateName,
		StartedAt:     time.Now().UTC(),
		SupervisorPID: os.Getpid(),
		Agents:        make([]upRuntimeStateAgent, 0, len(roles)),
	}
	defer func() {
		_ = removeUpRuntimeState(projectDir)
		_ = runWithCleanupTimeout(5*time.Second, func(cleanupCtx context.Context) error {
			return deregisterUserPseudoAgent(cleanupCtx, app.db, state.UserAgent)
		})
	}()

	for _, role := range roles {
		agentState, cmd, err := startRegisteredRole(projectDir, projectName, role)
		if err != nil {
			_ = shutdownChildCommands(children, true, 5*time.Second)
			return fmt.Errorf("cli.runUpSupervisor: start role %s: %w", role.Name, err)
		}
		children = append(children, cmd)
		state.Agents = append(state.Agents, agentState)
	}

	userAgent, err := registerUserPseudoAgent(ctx, app.db, state.SupervisorPID, projectName)
	if err != nil {
		_ = shutdownChildCommands(children, true, 5*time.Second)
		return fmt.Errorf("cli.runUpSupervisor: register user agent: %w", err)
	}
	state.UserAgent = userAgent

	if err := writeUpRuntimeState(projectDir, state); err != nil {
		_ = shutdownChildCommands(children, true, 5*time.Second)
		return fmt.Errorf("cli.runUpSupervisor: write runtime state: %w", err)
	}

	type childExit struct {
		pid int
		err error
	}
	exitCh := make(chan childExit, len(children))
	for _, child := range children {
		go func(child *exec.Cmd) {
			exitCh <- childExit{pid: child.Process.Pid, err: child.Wait()}
		}(child)
	}

	active := make(map[int]upRuntimeStateAgent, len(state.Agents))
	for _, agentState := range state.Agents {
		active[agentState.PID] = agentState
	}
	healthTicker := time.NewTicker(upSupervisorHealthCheckInterval)
	defer healthTicker.Stop()

	for len(active) > 0 {
		select {
		case <-ctx.Done():
			if err := shutdownChildCommands(children, false, 5*time.Second); err != nil {
				return fmt.Errorf("cli.runUpSupervisor: shutdown children: %w", err)
			}
			return nil
		case <-healthTicker.C:
			staleAgents, err := collectStaleRuntimeAgents(ctx, app.db, flattenActiveAgents(active), 30*time.Second, time.Now().UTC())
			if err != nil {
				return fmt.Errorf("cli.runUpSupervisor: collect stale agents: %w", err)
			}
			if len(staleAgents) > 0 {
				_ = shutdownChildCommands(children, true, 5*time.Second)
				return fmt.Errorf("cli.runUpSupervisor: stale child agents detected: %d", len(staleAgents))
			}
		case exited := <-exitCh:
			delete(active, exited.pid)
			state.Agents = flattenActiveAgents(active)
			if len(state.Agents) == 0 {
				return nil
			}
			if err := writeUpRuntimeState(projectDir, state); err != nil {
				return fmt.Errorf("cli.runUpSupervisor: update runtime state: %w", err)
			}
		}
	}

	return nil
}

func registerUserPseudoAgent(ctx context.Context, database *db.DB, supervisorPID int, projectName string) (*upRuntimeStateAgent, error) {
	staleUser, err := database.FindAgentByNameAndProject(ctx, "user", projectName)
	if err == nil {
		if err := database.DeleteAgent(ctx, staleUser.ID); err != nil {
			return nil, fmt.Errorf("cli.registerUserPseudoAgent: delete stale user: %w", err)
		}
	} else if !errors.Is(err, db.ErrAgentNotFound) {
		return nil, fmt.Errorf("cli.registerUserPseudoAgent: find stale user: %w", err)
	}

	userAgent := &db.Agent{
		Name:    "user",
		Type:    "human",
		PID:     supervisorPID,
		Project: projectName,
		Status:  "alive",
	}
	if err := database.InsertAgent(ctx, userAgent); err != nil {
		return nil, fmt.Errorf("cli.registerUserPseudoAgent: insert: %w", err)
	}

	return &upRuntimeStateAgent{
		Role:    "user",
		Name:    userAgent.Name,
		Type:    userAgent.Type,
		PID:     userAgent.PID,
		AgentID: userAgent.ID,
		Project: userAgent.Project,
	}, nil
}

func deregisterUserPseudoAgent(ctx context.Context, database *db.DB, userAgent *upRuntimeStateAgent) error {
	if database == nil || userAgent == nil || userAgent.AgentID == "" {
		return nil
	}
	if err := database.DeleteAgent(ctx, userAgent.AgentID); err != nil && !errors.Is(err, db.ErrAgentNotFound) {
		return fmt.Errorf("cli.deregisterUserPseudoAgent: %w", err)
	}
	return nil
}

func flattenActiveAgents(active map[int]upRuntimeStateAgent) []upRuntimeStateAgent {
	agents := make([]upRuntimeStateAgent, 0, len(active))
	for _, agentState := range active {
		agents = append(agents, agentState)
	}
	sort.Slice(agents, func(i, j int) bool { return agents[i].Role < agents[j].Role })
	return agents
}

func startRegisteredRole(projectDir, projectName string, role templateManifestRole) (upRuntimeStateAgent, *exec.Cmd, error) {
	exePath, err := os.Executable()
	if err != nil {
		return upRuntimeStateAgent{}, nil, fmt.Errorf("cli.startRegisteredRole: executable: %w", err)
	}
	args := []string{"--json"}
	if projectName != "" {
		args = append(args, "--project", projectName)
	}
	args = append(args, "register", "--name", role.AgentName, "--type", role.AgentType, "--workdir", projectDir)
	cmd := exec.Command(exePath, args...)
	cmd.Dir = projectDir
	cmd.Env = os.Environ()
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return upRuntimeStateAgent{}, nil, fmt.Errorf("cli.startRegisteredRole: stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return upRuntimeStateAgent{}, nil, fmt.Errorf("cli.startRegisteredRole: stderr pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return upRuntimeStateAgent{}, nil, fmt.Errorf("cli.startRegisteredRole: start: %w", err)
	}
	go io.Copy(io.Discard, stderr)
	meta, err := decodeRegisterMetadata(stdout)
	if err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return upRuntimeStateAgent{}, nil, fmt.Errorf("cli.startRegisteredRole: decode metadata: %w", err)
	}
	return upRuntimeStateAgent{
		Role:       role.Name,
		Name:       role.AgentName,
		Type:       role.AgentType,
		PID:        cmd.Process.Pid,
		AgentID:    stringValue(meta["id"]),
		SocketPath: stringValue(meta["socket_path"]),
		Project:    stringValue(meta["project"]),
	}, cmd, nil
}

func decodeRegisterMetadata(r io.Reader) (map[string]any, error) {
	var meta map[string]any
	if err := json.NewDecoder(r).Decode(&meta); err != nil {
		return nil, fmt.Errorf("cli.decodeRegisterMetadata: %w", err)
	}
	return meta, nil
}

func stringValue(v any) string {
	s, _ := v.(string)
	return s
}

func stopRuntimeState(projectDir string, state upRuntimeState, onlyRoles []string, force bool, timeout time.Duration) ([]string, []string, error) {
	targets := state.Agents
	if len(onlyRoles) > 0 {
		selected := make([]upRuntimeStateAgent, 0, len(onlyRoles))
		byRole := make(map[string]upRuntimeStateAgent, len(state.Agents))
		for _, agentState := range state.Agents {
			byRole[agentState.Role] = agentState
		}
		for _, role := range onlyRoles {
			agentState, ok := byRole[role]
			if !ok {
				return nil, nil, fmt.Errorf("unknown running role %q", role)
			}
			selected = append(selected, agentState)
		}
		targets = selected
	}

	stopped := make([]string, 0, len(targets))
	for _, agentState := range targets {
		if err := signalProcess(agentState.PID, force); err != nil && !errors.Is(err, os.ErrProcessDone) {
			return nil, nil, fmt.Errorf("signal %s: %w", agentState.Role, err)
		}
		stopped = append(stopped, agentState.Role)
	}

	if len(onlyRoles) == 0 {
		if err := signalProcess(state.SupervisorPID, force); err != nil && !errors.Is(err, os.ErrProcessDone) {
			return nil, nil, fmt.Errorf("signal supervisor: %w", err)
		}
		if force {
			if err := runWithCleanupTimeout(5*time.Second, func(cleanupCtx context.Context) error {
				return deregisterUserPseudoAgent(cleanupCtx, app.db, state.UserAgent)
			}); err != nil {
				return nil, nil, err
			}
			if err := removeUpRuntimeState(projectDir); err != nil {
				return nil, nil, err
			}
			return stopped, nil, nil
		}
		if err := waitForRuntimeStateRemoval(projectDir, timeout); err != nil {
			return nil, nil, err
		}
		if err := runWithCleanupTimeout(5*time.Second, func(cleanupCtx context.Context) error {
			return deregisterUserPseudoAgent(cleanupCtx, app.db, state.UserAgent)
		}); err != nil {
			return nil, nil, err
		}
		return stopped, nil, nil
	}

	for _, agentState := range targets {
		if err := waitForProcessExit(agentState.PID, timeout); err != nil && !force {
			return nil, nil, fmt.Errorf("wait for %s exit: %w", agentState.Role, err)
		}
	}

	remaining := make([]upRuntimeStateAgent, 0, len(state.Agents)-len(targets))
	removed := make(map[string]struct{}, len(targets))
	for _, role := range stopped {
		removed[role] = struct{}{}
	}
	remainingRoles := make([]string, 0, len(state.Agents)-len(targets))
	for _, agentState := range state.Agents {
		if _, ok := removed[agentState.Role]; ok {
			continue
		}
		remaining = append(remaining, agentState)
		remainingRoles = append(remainingRoles, agentState.Role)
	}
	if len(remaining) == 0 {
		if err := signalProcess(state.SupervisorPID, force); err == nil || errors.Is(err, os.ErrProcessDone) {
			_ = removeUpRuntimeState(projectDir)
		}
		return stopped, nil, nil
	}
	state.Agents = remaining
	if err := writeUpRuntimeState(projectDir, state); err != nil {
		return nil, nil, err
	}
	sort.Strings(stopped)
	sort.Strings(remainingRoles)
	return stopped, remainingRoles, nil
}

func shutdownChildCommands(children []*exec.Cmd, force bool, timeout time.Duration) error {
	for _, child := range children {
		if child == nil || child.Process == nil {
			continue
		}
		if err := signalProcess(child.Process.Pid, force); err != nil && !errors.Is(err, os.ErrProcessDone) {
			return err
		}
	}
	deadline := time.Now().Add(timeout)
	for _, child := range children {
		if child == nil || child.Process == nil {
			continue
		}
		remaining := time.Until(deadline)
		if remaining < 0 {
			remaining = 0
		}
		if err := waitForProcessExit(child.Process.Pid, remaining); err != nil && !force {
			return err
		}
	}
	return nil
}

func runWithCleanupTimeout(timeout time.Duration, fn func(context.Context) error) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return fn(ctx)
}

func cleanupStaleRuntimeState(projectDir string, state upRuntimeState) error {
	for _, agentState := range state.Agents {
		if agentState.SocketPath == "" {
			continue
		}
		if err := os.Remove(agentState.SocketPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("cleanup stale socket %s: %w", agentState.Role, err)
		}
	}
	if state.UserAgent != nil && state.UserAgent.SocketPath != "" {
		if err := os.Remove(state.UserAgent.SocketPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("cleanup stale user socket: %w", err)
		}
	}
	if err := removeUpRuntimeState(projectDir); err != nil {
		return err
	}
	return nil
}

func collectStaleRuntimeAgents(ctx context.Context, database *db.DB, agents []upRuntimeStateAgent, staleThreshold time.Duration, now time.Time) ([]upRuntimeStateAgent, error) {
	stale := make([]upRuntimeStateAgent, 0)
	for _, agentState := range agents {
		if agentState.AgentID == "" || agentState.Type == "human" {
			continue
		}
		agentRecord, err := database.FindAgentByID(ctx, agentState.AgentID)
		if err != nil {
			return nil, fmt.Errorf("collect stale agent %s: %w", agentState.Role, err)
		}
		if now.Sub(agentRecord.LastHeartbeat) <= staleThreshold {
			continue
		}
		if processAliveCheck(agentState.PID) {
			continue
		}
		stale = append(stale, agentState)
	}
	return stale, nil
}

func waitForRuntimeStateRemoval(projectDir string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		state, _, err := loadUpRuntimeState(projectDir)
		if err != nil {
			return err
		}
		if state.SupervisorPID == 0 {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("timed out waiting for runtime state removal")
}

func waitForProcessExit(pid int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !processAliveCheck(pid) {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !processAliveCheck(pid) {
		return nil
	}
	return fmt.Errorf("process %d still running", pid)
}

func signalProcess(pid int, force bool) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	if force {
		return proc.Signal(os.Kill)
	}
	return proc.Signal(os.Interrupt)
}
