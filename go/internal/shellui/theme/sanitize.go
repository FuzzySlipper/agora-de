package theme

import (
	"fmt"
	"sort"
	"strings"
)

var forbiddenCSSFragments = []string{
	"position:",
	"display:",
	"@import",
	"url(",
	"behavior:",
	"expression(",
}

func ValidateToken(name string, value string) error {
	name = strings.TrimSpace(name)
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(name, "--agora-") {
		return fmt.Errorf("theme token %q is not an --agora-* token", name)
	}
	if !KnownTokenNames()[name] {
		return fmt.Errorf("theme token %q is not a known agora-de theme token", name)
	}
	if value == "" {
		return fmt.Errorf("theme token %q has empty value", name)
	}
	if strings.ContainsAny(value, "{};") {
		return fmt.Errorf("theme token %q contains unsafe punctuation", name)
	}
	if err := ValidateSafeVisualCSS(value); err != nil {
		return fmt.Errorf("theme token %q has unsafe value: %w", name, err)
	}
	return nil
}

func KnownTokenNames() map[string]bool {
	definitions := DefaultTokenDefinitions()
	names := make(map[string]bool, len(definitions))
	for _, definition := range definitions {
		names[definition.Name] = true
	}
	return names
}

func SafeTokenCSS(tokens map[string]string) (string, error) {
	var builder strings.Builder
	builder.WriteString(":root {\n")
	names := make([]string, 0, len(tokens))
	for name := range tokens {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		value := tokens[name]
		if err := ValidateToken(name, value); err != nil {
			return "", err
		}
		builder.WriteString("  ")
		builder.WriteString(strings.TrimSpace(name))
		builder.WriteString(": ")
		builder.WriteString(strings.TrimSpace(value))
		builder.WriteString(";\n")
	}
	builder.WriteString("}\n")
	return builder.String(), nil
}

func ValidateSafeVisualCSS(css string) error {
	lower := strings.ToLower(css)
	for _, fragment := range forbiddenCSSFragments {
		if strings.Contains(lower, fragment) {
			return fmt.Errorf("theme CSS contains forbidden fragment %q", fragment)
		}
	}
	return nil
}
