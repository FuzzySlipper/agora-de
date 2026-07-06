package appcatalog

import (
	"bufio"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Entry struct {
	ID             string
	Type           string
	Name           string
	Exec           string
	ExecTokens     []string
	ExecSupported  bool
	Icon           string
	Categories     []string
	StartupWMClass string
	NoDisplay      bool
	Hidden         bool
}

func ParseDesktopEntry(id string, reader io.Reader) (Entry, error) {
	scanner := bufio.NewScanner(reader)
	inDesktopEntry := false
	values := map[string]string{}

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			inDesktopEntry = line == "[Desktop Entry]"
			continue
		}
		if !inDesktopEntry {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return Entry{}, fmt.Errorf("invalid desktop entry line %q", line)
		}
		key = strings.TrimSpace(key)
		if strings.Contains(key, "[") {
			continue
		}
		values[key] = strings.TrimSpace(value)
	}
	if err := scanner.Err(); err != nil {
		return Entry{}, fmt.Errorf("scan desktop entry: %w", err)
	}

	tokens, execSupported := NormalizeExec(values["Exec"])
	entry := Entry{
		ID:             strings.TrimSpace(id),
		Type:           strings.TrimSpace(values["Type"]),
		Name:           values["Name"],
		Exec:           values["Exec"],
		ExecTokens:     tokens,
		ExecSupported:  execSupported,
		Icon:           values["Icon"],
		Categories:     ParseCategories(values["Categories"]),
		StartupWMClass: strings.TrimSpace(values["StartupWMClass"]),
		NoDisplay:      parseDesktopBool(values["NoDisplay"]),
		Hidden:         parseDesktopBool(values["Hidden"]),
	}
	if entry.ID == "" {
		return Entry{}, fmt.Errorf("desktop entry missing id")
	}
	if entry.Type == "" {
		return Entry{}, fmt.Errorf("desktop entry %q missing Type", entry.ID)
	}
	if entry.Name == "" {
		return Entry{}, fmt.Errorf("desktop entry %q missing Name", entry.ID)
	}
	if entry.Type == "Application" && entry.Exec == "" {
		return Entry{}, fmt.Errorf("desktop entry %q missing Exec", entry.ID)
	}
	return entry, nil
}

func ImportDesktopEntries(roots ...string) (*Catalog, error) {
	catalog := NewCatalog()
	seen := map[string]bool{}

	for _, root := range roots {
		root = strings.TrimSpace(root)
		if root == "" {
			continue
		}
		if _, err := os.Stat(root); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("stat desktop entry root %q: %w", root, err)
		}

		paths := []string{}
		if err := filepath.WalkDir(root, func(path string, dirEntry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if dirEntry.IsDir() {
				return nil
			}
			if filepath.Ext(path) == ".desktop" {
				paths = append(paths, path)
			}
			return nil
		}); err != nil {
			return nil, fmt.Errorf("walk desktop entry root %q: %w", root, err)
		}
		sort.Strings(paths)

		for _, path := range paths {
			id, err := desktopEntryID(root, path)
			if err != nil {
				return nil, err
			}
			if seen[id] {
				continue
			}
			entry, err := parseDesktopEntryFile(id, path)
			if err != nil {
				continue
			}
			if entry.Type != "Application" {
				continue
			}
			catalog.Add(entry)
			seen[id] = true
		}
	}

	return catalog, nil
}

func parseDesktopEntryFile(id string, path string) (Entry, error) {
	file, err := os.Open(path)
	if err != nil {
		return Entry{}, fmt.Errorf("open desktop entry %q: %w", path, err)
	}
	defer file.Close()
	return ParseDesktopEntry(id, file)
}

func desktopEntryID(root string, path string) (string, error) {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return "", fmt.Errorf("desktop entry id for %q: %w", path, err)
	}
	return strings.ReplaceAll(filepath.ToSlash(relative), "/", "-"), nil
}

func (entry Entry) Visible() bool {
	return entry.Type == "Application" && !entry.NoDisplay && !entry.Hidden
}

func (entry Entry) Launchable() bool {
	return entry.Visible() && entry.ExecSupported && len(entry.ExecTokens) > 0
}

func NormalizeExec(execValue string) ([]string, bool) {
	tokens, ok := splitExec(execValue)
	if !ok {
		return nil, false
	}
	normalized := make([]string, 0, len(tokens))
	for _, token := range tokens {
		value, keep, ok := normalizeExecToken(token)
		if !ok {
			return nil, false
		}
		if keep && value != "" {
			normalized = append(normalized, value)
		}
	}
	return normalized, len(normalized) > 0
}

func normalizeExecToken(token string) (string, bool, bool) {
	switch token {
	case "%f", "%F", "%u", "%U", "%i", "%c", "%k":
		return "", false, true
	}
	var builder strings.Builder
	for i := 0; i < len(token); i++ {
		if token[i] != '%' {
			builder.WriteByte(token[i])
			continue
		}
		if i+1 >= len(token) {
			return "", false, false
		}
		i++
		switch token[i] {
		case '%':
			builder.WriteByte('%')
		case 'f', 'F', 'u', 'U', 'i', 'c', 'k':
		default:
			return "", false, false
		}
	}
	return builder.String(), true, true
}

func splitExec(value string) ([]string, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, true
	}
	tokens := []string{}
	var builder strings.Builder
	inQuote := false
	escaping := false
	hasToken := false

	for _, char := range value {
		switch {
		case escaping:
			builder.WriteRune(char)
			escaping = false
			hasToken = true
		case char == '\\':
			escaping = true
			hasToken = true
		case char == '"':
			inQuote = !inQuote
			hasToken = true
		case !inQuote && (char == ' ' || char == '\t' || char == '\n' || char == '\r'):
			if hasToken {
				tokens = append(tokens, builder.String())
				builder.Reset()
				hasToken = false
			}
		default:
			builder.WriteRune(char)
			hasToken = true
		}
	}
	if escaping || inQuote {
		return nil, false
	}
	if hasToken {
		tokens = append(tokens, builder.String())
	}
	return tokens, true
}

func parseDesktopBool(value string) bool {
	return strings.EqualFold(strings.TrimSpace(value), "true")
}

func ParseCategories(value string) []string {
	seen := map[string]bool{}
	categories := []string{}
	for _, item := range strings.Split(value, ";") {
		item = strings.TrimSpace(item)
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		categories = append(categories, item)
	}
	return categories
}
