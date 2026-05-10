package cmd

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/manifoldco/promptui"
)

const (
	colorReset  = "\033[0m"
	colorCyan   = "\033[36m"
	colorBlue   = "\033[34m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorDim    = "\033[2m"
)

type promptField struct {
	Name        string
	Description string
	Default     string
	Required    bool
	Type        string
}

var installPromptTemplates = &promptui.PromptTemplates{
	Prompt:  "{{ . }} ",
	Valid:   "{{ . | cyan }} ",
	Invalid: "{{ . | red }} ",
	Success: "{{ . | green }} ",
}

func printInstallHeader(manifest installManifest, source installSource) {
	displayName := nestedString(manifest.Tool, "displayName")
	name := nestedString(manifest.Tool, "name")

	fmt.Printf("\n%s%sTool Install%s\n", colorBlue, promptui.IconSelect, colorReset)

	switch {
	case displayName != "" && name != "":
		fmt.Printf("%sName:%s %s%s%s %s(%s)%s\n", colorCyan, colorReset, colorGreen, displayName, colorReset, colorDim, name, colorReset)
	case name != "":
		fmt.Printf("%sName:%s %s%s%s\n", colorCyan, colorReset, colorGreen, name, colorReset)
	default:
		fmt.Printf("%sName:%s %stool%s\n", colorCyan, colorReset, colorGreen, colorReset)
	}

	fmt.Printf("%sSource:%s %s%s%s\n", colorCyan, colorReset, source.Label, sourceTargetSuffix(source.Target), colorReset)
	fmt.Printf("%sHint:%s press Enter to keep the current/default value.\n\n", colorCyan, colorReset)
}

func printSecretsNote(manifest installManifest) {
	permissions, _ := manifest.Tool["permissions"].(map[string]any)
	secrets := jsonStringSlice(permissions["secrets"])
	if len(secrets) == 0 {
		return
	}

	sort.Strings(secrets)
	fmt.Printf("%sSecrets:%s provided secrets will be resolved from environment variables. Make sure your runtime exposes these values: %s%s%s\n\n", colorYellow, colorReset, colorDim, strings.Join(secrets, ", "), colorReset)
}

func promptConfigFields(properties map[string]any, required []string, defaults map[string]any, prefix string, interactive bool) (map[string]any, error) {
	config := make(map[string]any)
	requiredSet := make(map[string]struct{}, len(required))
	for _, key := range required {
		requiredSet[key] = struct{}{}
	}

	keys := sortedMapKeys(properties)
	for _, key := range keys {
		prop, ok := properties[key].(map[string]any)
		if !ok {
			continue
		}

		if isSecretConfigField(key, prop) {
			continue
		}

		currentPrefix := key
		if prefix != "" {
			currentPrefix = prefix + "." + key
		}

		defaultValue, hasDefaultFromConfig := defaults[key]
		_, requiredField := requiredSet[key]

		if hasNestedProperties(prop) {
			childDefaults, _ := defaultValue.(map[string]any)
			childConfig, err := promptConfigFields(childProperties(prop), jsonStringSlice(prop["required"]), childDefaults, currentPrefix, interactive)
			if err != nil {
				return nil, err
			}
			if len(childConfig) > 0 {
				config[key] = childConfig
				continue
			}
			if requiredField {
				return nil, fmt.Errorf("missing required config for %s", currentPrefix)
			}
			continue
		}

		value, keep, err := resolveConfigValue(currentPrefix, prop, defaultValue, hasDefaultFromConfig, requiredField, interactive)
		if err != nil {
			return nil, err
		}
		if keep {
			config[key] = value
		}
	}

	return config, nil
}

func resolveConfigValue(name string, prop map[string]any, configDefault any, hasConfigDefault, required, interactive bool) (any, bool, error) {
	defaultValue, hasDefault := effectiveDefaultValue(prop, configDefault, hasConfigDefault)
	if !interactive {
		switch {
		case hasDefault:
			return defaultValue, true, nil
		case required:
			return nil, false, fmt.Errorf("missing required config for %s in non-interactive mode", name)
		default:
			return nil, false, nil
		}
	}

	field := buildPromptField(name, prop, defaultValue, hasDefault, required)
	switch field.Type {
	case "boolean":
		return promptBool(field, defaultValue, hasDefault, required)
	default:
		return promptText(field, prop, defaultValue, hasDefault, required)
	}
}

func buildPromptField(name string, prop map[string]any, defaultValue any, hasDefault, required bool) promptField {
	desc, _ := prop["description"].(string)
	field := promptField{
		Name:        name,
		Description: desc,
		Required:    required,
		Type:        schemaType(prop),
	}
	if hasDefault {
		field.Default = formatPromptValue(defaultValue)
	}
	return field
}

func promptText(field promptField, prop map[string]any, defaultValue any, hasDefault, required bool) (any, bool, error) {
	printPromptFieldBlock(field)
	prompt := promptui.Prompt{
		Label:     ">",
		Default:   field.Default,
		Templates: installPromptTemplates,
		Validate: func(input string) error {
			if strings.TrimSpace(input) == "" {
				if hasDefault || !required {
					return nil
				}
				return fmt.Errorf("value required")
			}
			_, err := parsePromptValue(input, prop)
			return err
		},
	}

	result, err := prompt.Run()
	if err != nil {
		return nil, false, err
	}

	result = strings.TrimSpace(result)
	if result == "" {
		switch {
		case hasDefault:
			return defaultValue, true, nil
		default:
			return nil, false, nil
		}
	}

	value, parseErr := parsePromptValue(result, prop)
	if parseErr != nil {
		return nil, false, parseErr
	}
	return value, true, nil
}

func promptBool(field promptField, defaultValue any, hasDefault, required bool) (any, bool, error) {
	printPromptFieldBlock(field)
	items := []string{"true", "false"}
	defaultIndex := 0

	if hasDefault {
		if current, ok := boolString(defaultValue); ok && current == "false" {
			defaultIndex = 1
		}
	}

	selectPrompt := promptui.Select{
		Label:             "Select value",
		Items:             items,
		CursorPos:         defaultIndex,
		HideSelected:      true,
		Size:              len(items),
		StartInSearchMode: false,
		Templates: &promptui.SelectTemplates{
			Label:    "{{ . }}",
			Active:   fmt.Sprintf("%s▸ {{ . | cyan }}%s", colorCyan, colorReset),
			Inactive: "  {{ . }}",
			Selected: fmt.Sprintf("%s✓ {{ . }}%s", colorGreen, colorReset),
		},
	}

	_, result, err := selectPrompt.Run()
	if err != nil {
		return nil, false, err
	}

	if result == "" {
		switch {
		case hasDefault:
			return defaultValue, true, nil
		case required:
			return nil, false, fmt.Errorf("missing required config for %s", field.Name)
		default:
			return nil, false, nil
		}
	}

	return result == "true", true, nil
}

func printPromptFieldBlock(field promptField) {
	fmt.Println()
	fmt.Printf("%s[%s]%s %s%s%s\n", colorCyan, promptFieldTypeLabel(field), colorReset, colorGreen, field.Name, colorReset)
	if field.Description != "" {
		fmt.Println(field.Description)
	}
	fmt.Printf("%s.\n", promptFieldHint(field))
	if field.Default != "" {
		fmt.Printf("%sDefault:%s %s\n", colorDim, colorReset, field.Default)
	}
}

func promptFieldTypeLabel(field promptField) string {
	if field.Type == "" {
		if field.Required {
			return "required"
		}
		return "optional"
	}
	return field.Type
}

func promptFieldHint(field promptField) string {
	parts := make([]string, 0, 2)
	if field.Required {
		parts = append(parts, colorYellow+"Required"+colorReset)
	} else {
		parts = append(parts, colorDim+"Optional"+colorReset)
	}
	if field.Type == "object" || field.Type == "array" {
		parts = append(parts, "enter JSON")
	}
	return strings.Join(parts, ". ")
}

func effectiveDefaultValue(prop map[string]any, configDefault any, hasConfigDefault bool) (any, bool) {
	if hasConfigDefault {
		return configDefault, true
	}
	defaultValue, hasSchemaDefault := prop["default"]
	if hasSchemaDefault {
		return defaultValue, true
	}
	return nil, false
}

func parsePromptValue(input string, prop map[string]any) (any, error) {
	switch schemaType(prop) {
	case "", "string":
		return input, nil
	case "integer":
		value, err := strconv.ParseInt(input, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("expected integer")
		}
		return value, nil
	case "number":
		value, err := strconv.ParseFloat(input, 64)
		if err != nil {
			return nil, fmt.Errorf("expected number")
		}
		return value, nil
	case "boolean":
		value, err := strconv.ParseBool(strings.ToLower(input))
		if err != nil {
			return nil, fmt.Errorf("expected boolean")
		}
		return value, nil
	case "array", "object":
		var value any
		if err := json.Unmarshal([]byte(input), &value); err != nil {
			return nil, fmt.Errorf("expected valid JSON for %s", schemaType(prop))
		}
		return value, nil
	default:
		return input, nil
	}
}

func formatPromptValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case fmt.Stringer:
		return typed.String()
	default:
		data, err := json.Marshal(typed)
		if err != nil {
			return fmt.Sprintf("%v", value)
		}
		return string(data)
	}
}

func sortedMapKeys(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func boolString(value any) (string, bool) {
	switch typed := value.(type) {
	case bool:
		if typed {
			return "true", true
		}
		return "false", true
	case string:
		lowered := strings.ToLower(strings.TrimSpace(typed))
		if lowered == "true" || lowered == "false" {
			return lowered, true
		}
	}
	return "", false
}

func sourceTargetSuffix(target string) string {
	if target == "" {
		return ""
	}
	return "  " + colorDim + target
}
