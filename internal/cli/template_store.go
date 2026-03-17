package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"github.com/malleus35/agentcom/internal/onboard"
	"github.com/malleus35/agentcom/internal/task"
)

func saveCustomTemplate(projectDir string, definition templateDefinition) (string, error) {
	return writeCustomTemplate(projectDir, definition, writeModeCreate)
}

func writeCustomTemplate(projectDir string, definition templateDefinition, mode writeMode) (string, error) {
	if err := validateCustomTemplateDefinition(definition); err != nil {
		return "", fmt.Errorf("validate custom template: %w", err)
	}

	baseDir := filepath.Join(projectDir, ".agentcom", "templates", definition.Name)
	commonPath := filepath.Join(baseDir, "COMMON.md")
	manifestPath := filepath.Join(baseDir, "template.json")

	commonContent := renderTemplateCommonContent(definition)
	if err := writeScaffoldFile(commonPath, commonContent, mode); err != nil {
		return "", fmt.Errorf("write custom template common markdown: %w", err)
	}

	manifestContent, err := renderTemplateManifest(definition)
	if err != nil {
		return "", fmt.Errorf("render custom template manifest: %w", err)
	}
	if err := writeScaffoldFile(manifestPath, manifestContent, mode); err != nil {
		return "", fmt.Errorf("write custom template manifest: %w", err)
	}

	return baseDir, nil
}

func loadCustomTemplate(projectDir string, name string) (templateDefinition, string, error) {
	manifestPath := filepath.Join(projectDir, ".agentcom", "templates", name, "template.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			return templateDefinition{}, "", fmt.Errorf("custom template %q not found", name)
		}
		return templateDefinition{}, "", fmt.Errorf("read custom template manifest: %w", err)
	}

	var definition templateDefinition
	if err := json.Unmarshal(data, &definition); err != nil {
		return templateDefinition{}, "", fmt.Errorf("unmarshal custom template manifest: %w", err)
	}

	baseDir := filepath.Dir(manifestPath)
	commonPath := filepath.Join(baseDir, "COMMON.md")
	commonData, err := os.ReadFile(commonPath)
	if err != nil {
		return templateDefinition{}, "", fmt.Errorf("read custom template common markdown: %w", err)
	}
	definition.CommonBody = extractTemplateCommonBody(string(commonData), definition.CommonTitle)
	return definition, baseDir, nil
}

func loadCustomTemplates(projectDir string) ([]templateDefinition, error) {
	paths, err := filepath.Glob(filepath.Join(projectDir, ".agentcom", "templates", "*", "template.json"))
	if err != nil {
		return nil, fmt.Errorf("glob custom templates: %w", err)
	}

	definitions := make([]templateDefinition, 0, len(paths))
	for _, manifestPath := range paths {
		data, err := os.ReadFile(manifestPath)
		if err != nil {
			return nil, fmt.Errorf("read custom template manifest: %w", err)
		}

		var definition templateDefinition
		if err := json.Unmarshal(data, &definition); err != nil {
			return nil, fmt.Errorf("unmarshal custom template manifest: %w", err)
		}
		if issues := validateCommunicationGraph(definition.Roles); hasGraphErrors(issues) {
			messages := make([]string, 0, len(issues))
			for _, issue := range issues {
				if issue.Severity == "error" {
					messages = append(messages, issue.Message)
				}
			}
			return nil, fmt.Errorf("custom template manifest has communication graph errors: %s", strings.Join(messages, "; "))
		}

		commonPath := filepath.Join(filepath.Dir(manifestPath), "COMMON.md")
		commonData, err := os.ReadFile(commonPath)
		if err != nil {
			return nil, fmt.Errorf("read custom template common markdown: %w", err)
		}
		definition.CommonBody = extractTemplateCommonBody(string(commonData), definition.CommonTitle)
		definitions = append(definitions, definition)
	}

	sort.Slice(definitions, func(i, j int) bool {
		return definitions[i].Name < definitions[j].Name
	})
	return definitions, nil
}

func mergeTemplateDefinitions(builtIn []templateDefinition, custom []templateDefinition) []templateDefinition {
	merged := make([]templateDefinition, 0, len(builtIn)+len(custom))
	seen := make(map[string]struct{}, len(builtIn)+len(custom))

	for _, definition := range builtIn {
		merged = append(merged, definition)
		seen[definition.Name] = struct{}{}
	}

	customSorted := append([]templateDefinition(nil), custom...)
	sort.Slice(customSorted, func(i, j int) bool {
		return customSorted[i].Name < customSorted[j].Name
	})
	for _, definition := range customSorted {
		if _, ok := seen[definition.Name]; ok {
			continue
		}
		merged = append(merged, definition)
		seen[definition.Name] = struct{}{}
	}

	return merged
}

func allTemplateDefinitions(projectDir string) ([]templateDefinition, error) {
	custom, err := loadCustomTemplates(projectDir)
	if err != nil {
		if errorsIsNotExist(err) {
			return builtInTemplateDefinitions(), nil
		}
		return nil, err
	}
	return mergeTemplateDefinitions(builtInTemplateDefinitions(), custom), nil
}

func validateCustomTemplateDefinition(definition templateDefinition) error {
	if err := validateSkillName(definition.Name); err != nil {
		return err
	}
	for _, builtIn := range builtInTemplateDefinitions() {
		if builtIn.Name == definition.Name {
			if reflect.DeepEqual(normalizeTemplateDefinition(definition), normalizeTemplateDefinition(builtIn)) {
				break
			}
			return newUserError(
				fmt.Sprintf("Template name %q conflicts with a built-in template", definition.Name),
				"Built-in template names are reserved and cannot be reused for custom templates.",
				"Choose a different name or inspect built-ins with `agentcom agents template --list`.",
			)
		}
	}
	if strings.TrimSpace(definition.Description) == "" {
		return newUserError(
			"Template description is missing",
			"Every template needs a short description so users can identify it later.",
			"Add a non-empty description in your template file or wizard input, then retry `agentcom init --from-file <path>`.",
		)
	}
	if strings.TrimSpace(definition.CommonTitle) == "" {
		return newUserError(
			"Template common title is missing",
			"The shared COMMON.md heading cannot be empty.",
			"Set `common_title` in the template definition before importing it again.",
		)
	}
	if strings.TrimSpace(definition.CommonBody) == "" {
		return newUserError(
			"Template common body is missing",
			"The shared COMMON.md content cannot be empty.",
			"Add shared instructions to the template definition, then retry the import.",
		)
	}
	if len(definition.Roles) == 0 {
		return newUserError(
			"Template has no roles",
			"A template must define at least one role to scaffold agent instructions.",
			"Add at least one role in the template definition, then retry the import.",
		)
	}
	if definition.ReviewPolicy != nil {
		if err := definition.ReviewPolicy.Validate(); err != nil {
			return newUserError(
				"Template review policy is invalid",
				err.Error(),
				"Fix `review_policy` and retry the template import or wizard flow.",
			)
		}
	}
	for _, role := range definition.Roles {
		if strings.TrimSpace(role.Name) == "" {
			return newUserError(
				"Template role name is missing",
				"Each template role needs a stable name.",
				"Fill in the role name and retry the template import or wizard flow.",
			)
		}
		if strings.TrimSpace(role.Description) == "" {
			return newUserError(
				fmt.Sprintf("Template role %q is missing a description", role.Name),
				"Each role needs a description for scaffolded documentation.",
				"Add a role description and retry the template import or wizard flow.",
			)
		}
		if strings.TrimSpace(role.AgentName) == "" {
			return newUserError(
				fmt.Sprintf("Template role %q is missing an agent name", role.Name),
				"Each role needs a concrete agent name for generated commands.",
				"Add the role's `agent_name` and retry the template import or wizard flow.",
			)
		}
		if strings.TrimSpace(role.AgentType) == "" {
			return newUserError(
				fmt.Sprintf("Template role %q is missing an agent type", role.Name),
				"Each role needs an agent type for scaffolded registration examples.",
				"Add the role's `agent_type` and retry the template import or wizard flow.",
			)
		}
	}
	if issues := validateCommunicationGraph(definition.Roles); hasGraphErrors(issues) {
		messages := make([]string, 0, len(issues))
		for _, issue := range issues {
			if issue.Severity == "error" {
				messages = append(messages, issue.Message)
			}
		}
		return newUserError(
			"Template communication graph is invalid",
			strings.Join(messages, "; "),
			"Fix the role communication map and retry `agentcom init --from-file <path>`.",
		)
	}
	return nil
}

func normalizeTemplateDefinition(definition templateDefinition) templateDefinition {
	definition.Roles = append([]templateRole(nil), definition.Roles...)
	for i := range definition.Roles {
		definition.Roles[i].CommunicatesWith = append([]string(nil), definition.Roles[i].CommunicatesWith...)
		definition.Roles[i].Responsibilities = append([]string(nil), definition.Roles[i].Responsibilities...)
	}
	return definition
}

func extractTemplateCommonBody(content string, title string) string {
	prefix := fmt.Sprintf("# %s\n\n", title)
	trimmed := strings.TrimPrefix(content, prefix)
	return strings.TrimSpace(trimmed)
}

func errorsIsNotExist(err error) bool {
	if err == nil {
		return false
	}
	return os.IsNotExist(err)
}

func templateDefinitionFromOnboard(definition onboard.TemplateDefinition) templateDefinition {
	roles := make([]templateRole, 0, len(definition.Roles))
	for _, role := range definition.Roles {
		roles = append(roles, templateRole{
			Name:             role.Name,
			Description:      role.Description,
			AgentName:        role.AgentName,
			AgentType:        role.AgentType,
			CommunicatesWith: append([]string(nil), role.CommunicatesWith...),
			Responsibilities: append([]string(nil), role.Responsibilities...),
		})
	}

	return templateDefinition{
		Name:         definition.Name,
		Description:  definition.Description,
		Reference:    definition.Reference,
		CommonTitle:  definition.CommonTitle,
		CommonBody:   definition.CommonBody,
		ReviewPolicy: cloneReviewPolicy(definition.ReviewPolicy),
		Roles:        roles,
	}
}

func cloneReviewPolicy(policy *task.ReviewPolicy) *task.ReviewPolicy {
	if policy == nil {
		return nil
	}
	cloned := &task.ReviewPolicy{
		RequireReviewAbove: policy.RequireReviewAbove,
		DefaultReviewer:    policy.DefaultReviewer,
		Rules:              append([]task.ReviewPolicyRule(nil), policy.Rules...),
	}
	return cloned
}
