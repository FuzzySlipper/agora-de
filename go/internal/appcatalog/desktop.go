package appcatalog

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

type Entry struct {
	ID        string
	Name      string
	Exec      string
	Icon      string
	NoDisplay bool
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
		values[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
	if err := scanner.Err(); err != nil {
		return Entry{}, fmt.Errorf("scan desktop entry: %w", err)
	}

	entry := Entry{
		ID:        strings.TrimSpace(id),
		Name:      values["Name"],
		Exec:      values["Exec"],
		Icon:      values["Icon"],
		NoDisplay: strings.EqualFold(values["NoDisplay"], "true"),
	}
	if entry.ID == "" {
		return Entry{}, fmt.Errorf("desktop entry missing id")
	}
	if entry.Name == "" {
		return Entry{}, fmt.Errorf("desktop entry %q missing Name", entry.ID)
	}
	if entry.Exec == "" {
		return Entry{}, fmt.Errorf("desktop entry %q missing Exec", entry.ID)
	}
	return entry, nil
}

