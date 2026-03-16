package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

func newTemplateEditCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "edit <template-name> <add-role|remove-role> <role-name>",
		Short: "Edit an existing custom template",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			templateName := args[0]
			action := args[1]
			roleName := args[2]

			var (
				generated []string
				err       error
			)
			switch action {
			case "add-role":
				generated, err = addRoleToTemplate(templateName, roleName)
			case "remove-role":
				generated, err = removeRoleFromTemplate(templateName, roleName)
			default:
				return fmt.Errorf("cli.newTemplateEditCmd: unknown action %q", action)
			}
			if err != nil {
				return fmt.Errorf("cli.newTemplateEditCmd: %w", err)
			}

			if jsonOutput {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(map[string]any{
					"template":        templateName,
					"action":          action,
					"role":            roleName,
					"generated_files": generated,
				})
			}

			_, err = fmt.Fprintf(cmd.OutOrStdout(), "updated template %s: %s %s\n", templateName, action, roleName)
			return err
		},
	}
	return cmd
}

func addRoleToTemplate(templateName string, roleName string) ([]string, error) {
	if isBuiltInTemplateName(templateName) {
		return nil, fmt.Errorf("cannot edit built-in template %q", templateName)
	}

	projectDir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("getwd: %w", err)
	}
	definition, _, err := loadCustomTemplate(projectDir, templateName)
	if err != nil {
		return nil, err
	}
	for _, role := range definition.Roles {
		if role.Name == roleName {
			return nil, fmt.Errorf("role %q already exists in template %q", roleName, templateName)
		}
	}

	allRoleNames := make([]string, 0, len(definition.Roles)+1)
	for _, role := range definition.Roles {
		allRoleNames = append(allRoleNames, role.Name)
	}
	allRoleNames = append(allRoleNames, roleName)

	for i := range definition.Roles {
		if !containsString(definition.Roles[i].CommunicatesWith, roleName) {
			definition.Roles[i].CommunicatesWith = append(definition.Roles[i].CommunicatesWith, roleName)
		}
	}
	definition.Roles = append(definition.Roles, generateDefaultRole(roleName, allRoleNames))

	if _, err := writeCustomTemplate(projectDir, definition, writeModeOverwrite); err != nil {
		return nil, err
	}
	generated, err := writeTemplateScaffold(projectDir, templateName, writeModeOverwrite)
	if err != nil {
		return nil, fmt.Errorf("regenerate scaffold: %w", err)
	}
	return generated, nil
}

func removeRoleFromTemplate(templateName string, roleName string) ([]string, error) {
	if isBuiltInTemplateName(templateName) {
		return nil, fmt.Errorf("cannot edit built-in template %q", templateName)
	}

	projectDir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("getwd: %w", err)
	}
	definition, _, err := loadCustomTemplate(projectDir, templateName)
	if err != nil {
		return nil, err
	}

	filtered := make([]templateRole, 0, len(definition.Roles)-1)
	removed := false
	for _, role := range definition.Roles {
		if role.Name == roleName {
			removed = true
			continue
		}
		role.CommunicatesWith = removeString(role.CommunicatesWith, roleName)
		filtered = append(filtered, role)
	}
	if !removed {
		return nil, fmt.Errorf("role %q not found in template %q", roleName, templateName)
	}
	if len(filtered) == 0 {
		return nil, fmt.Errorf("template %q must keep at least one role", templateName)
	}
	definition.Roles = filtered

	if err := removeTemplateRoleSkillFiles(projectDir, templateName, roleName); err != nil {
		return nil, err
	}
	if _, err := writeCustomTemplate(projectDir, definition, writeModeOverwrite); err != nil {
		return nil, err
	}
	generated, err := writeTemplateScaffold(projectDir, templateName, writeModeOverwrite)
	if err != nil {
		return nil, fmt.Errorf("regenerate scaffold: %w", err)
	}
	return generated, nil
}

func removeTemplateRoleSkillFiles(projectDir string, templateName string, roleName string) error {
	targets, err := resolveTemplateSkillTargets("project", filepath.Join("agentcom", templateRoleSkillName(templateName, roleName)))
	if err != nil {
		return fmt.Errorf("resolve role skill targets: %w", err)
	}
	for _, target := range targets {
		if err := os.Remove(target.Path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove role skill file: %w", err)
		}
		if err := os.Remove(filepath.Dir(target.Path)); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove role skill directory: %w", err)
		}
	}
	return nil
}

func isBuiltInTemplateName(name string) bool {
	for _, definition := range builtInTemplateDefinitions() {
		if definition.Name == name {
			return true
		}
	}
	return false
}

func removeString(values []string, target string) []string {
	filtered := make([]string, 0, len(values))
	for _, value := range values {
		if value != target {
			filtered = append(filtered, value)
		}
	}
	return filtered
}
