package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/malleus35/agentcom/internal/onboard"
)

func saveCustomTemplate(projectDir string, definition templateDefinition) (string, error) {
	if err := validateCustomTemplateDefinition(definition); err != nil {
		return "", fmt.Errorf("validate custom template: %w", err)
	}

	baseDir := filepath.Join(projectDir, ".agentcom", "templates", definition.Name)
	commonPath := filepath.Join(baseDir, "COMMON.md")
	manifestPath := filepath.Join(baseDir, "template.json")

	commonContent := renderTemplateCommonContent(definition)
	if err := writeScaffoldFile(commonPath, commonContent); err != nil {
		return "", fmt.Errorf("write custom template common markdown: %w", err)
	}

	manifestContent, err := renderTemplateManifest(definition)
	if err != nil {
		return "", fmt.Errorf("render custom template manifest: %w", err)
	}
	if err := writeScaffoldFile(manifestPath, manifestContent); err != nil {
		return "", fmt.Errorf("write custom template manifest: %w", err)
	}

	return baseDir, nil
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
			return fmt.Errorf("template name %q conflicts with a built-in template", definition.Name)
		}
	}
	if strings.TrimSpace(definition.Description) == "" {
		return fmt.Errorf("template description is required")
	}
	if strings.TrimSpace(definition.CommonTitle) == "" {
		return fmt.Errorf("template common title is required")
	}
	if strings.TrimSpace(definition.CommonBody) == "" {
		return fmt.Errorf("template common body is required")
	}
	if len(definition.Roles) == 0 {
		return fmt.Errorf("at least one template role is required")
	}
	for _, role := range definition.Roles {
		if strings.TrimSpace(role.Name) == "" {
			return fmt.Errorf("template role name is required")
		}
		if strings.TrimSpace(role.Description) == "" {
			return fmt.Errorf("template role description is required")
		}
		if strings.TrimSpace(role.AgentName) == "" {
			return fmt.Errorf("template role agent name is required")
		}
		if strings.TrimSpace(role.AgentType) == "" {
			return fmt.Errorf("template role agent type is required")
		}
	}
	return nil
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
		Name:        definition.Name,
		Description: definition.Description,
		Reference:   definition.Reference,
		CommonTitle: definition.CommonTitle,
		CommonBody:  definition.CommonBody,
		Roles:       roles,
	}
}
