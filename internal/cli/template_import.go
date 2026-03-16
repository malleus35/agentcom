package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

func loadTemplateDefinitionFromFile(path string) (templateDefinition, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return templateDefinition{}, fmt.Errorf("read template file: %w", err)
	}

	var raw map[string]any
	if json.Valid(data) {
		if err := json.Unmarshal(data, &raw); err != nil {
			return templateDefinition{}, fmt.Errorf("parse template JSON: %w", err)
		}
	} else {
		if err := yaml.Unmarshal(data, &raw); err != nil {
			return templateDefinition{}, fmt.Errorf("parse template YAML: %w", err)
		}
	}

	definition, err := templateDefinitionFromMap(raw, path)
	if err != nil {
		return templateDefinition{}, err
	}
	if err := validateCustomTemplateDefinition(definition); err != nil {
		return templateDefinition{}, fmt.Errorf("validate imported template: %w", err)
	}
	return definition, nil
}

func templateDefinitionFromMap(raw map[string]any, sourcePath string) (templateDefinition, error) {
	name, err := requiredStringField(raw, "name")
	if err != nil {
		return templateDefinition{}, err
	}

	rolesValue, ok := raw["roles"]
	if !ok {
		return templateDefinition{}, fmt.Errorf("template roles are required")
	}
	roleList, ok := rolesValue.([]any)
	if !ok || len(roleList) == 0 {
		return templateDefinition{}, fmt.Errorf("template roles must be a non-empty list")
	}

	definition := templateDefinition{
		Name:        name,
		Description: optionalStringField(raw, "description", fmt.Sprintf("Custom template imported from %s.", filepath.Base(sourcePath))),
		Reference:   optionalStringField(raw, "reference", "file"),
		CommonTitle: optionalStringField(raw, "common_title", fmt.Sprintf("%s Template Common Instructions", titleWords(strings.ReplaceAll(name, "-", " ")))),
		CommonBody:  optionalStringField(raw, "common_body", defaultImportedCommonBody(name)),
	}

	if isStringRoleList(roleList) {
		roleNames, err := toStringSlice(roleList)
		if err != nil {
			return templateDefinition{}, err
		}
		definition.Roles = make([]templateRole, 0, len(roleNames))
		for _, roleName := range roleNames {
			definition.Roles = append(definition.Roles, generateDefaultRole(roleName, roleNames))
		}
		return definition, nil
	}

	roles := make([]templateRole, 0, len(roleList))
	for _, item := range roleList {
		roleMap, ok := item.(map[string]any)
		if !ok {
			return templateDefinition{}, fmt.Errorf("template roles must be either a list of names or a list of objects")
		}
		name, err := requiredStringField(roleMap, "name")
		if err != nil {
			return templateDefinition{}, fmt.Errorf("role: %w", err)
		}
		roles = append(roles, templateRole{
			Name:             name,
			Description:      optionalStringField(roleMap, "description", ""),
			AgentName:        optionalStringField(roleMap, "agent_name", ""),
			AgentType:        optionalStringField(roleMap, "agent_type", ""),
			CommunicatesWith: optionalStringSliceField(roleMap, "communicates_with"),
			Responsibilities: optionalStringSliceField(roleMap, "responsibilities"),
		})
	}
	definition.Roles = roles
	return definition, nil
}

func requiredStringField(values map[string]any, key string) (string, error) {
	value := optionalStringField(values, key, "")
	if strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("%s is required", key)
	}
	return value, nil
}

func optionalStringField(values map[string]any, key string, fallback string) string {
	value, ok := values[key]
	if !ok || value == nil {
		return fallback
	}
	text, ok := value.(string)
	if !ok {
		return fallback
	}
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return fallback
	}
	return trimmed
}

func optionalStringSliceField(values map[string]any, key string) []string {
	value, ok := values[key]
	if !ok || value == nil {
		return nil
	}
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	strings, err := toStringSlice(items)
	if err != nil {
		return nil
	}
	return strings
}

func isStringRoleList(items []any) bool {
	for _, item := range items {
		if _, ok := item.(string); !ok {
			return false
		}
	}
	return true
}

func toStringSlice(items []any) ([]string, error) {
	values := make([]string, 0, len(items))
	for _, item := range items {
		text, ok := item.(string)
		if !ok {
			return nil, fmt.Errorf("expected string list entry")
		}
		trimmed := strings.TrimSpace(text)
		if trimmed == "" {
			return nil, fmt.Errorf("string list entries must not be empty")
		}
		values = append(values, trimmed)
	}
	return values, nil
}

func defaultImportedCommonBody(name string) string {
	title := titleWords(strings.ReplaceAll(name, "-", " "))
	return strings.TrimSpace(fmt.Sprintf(`Use this template when you want a custom imported team definition.

- Treat the imported manifest as the source of truth for roles and communication paths.
- Run `+"`agentcom up`"+` after scaffold generation to start the managed team.
- Use `+"`agentcom down`"+` to stop the session cleanly.
- Keep role handoffs explicit with `+"`agentcom task create`"+` and `+"`agentcom send`"+`.
- Review template changes in version control before sharing them with the rest of the team.

This COMMON.md was generated from the imported template definition for %s.`, title))
}
