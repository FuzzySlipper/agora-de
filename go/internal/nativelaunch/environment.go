package nativelaunch

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const DefaultPath = "/usr/local/bin:/usr/bin:/bin"

var allowedExactEnvironment = map[string]struct{}{
	"DBUS_SESSION_BUS_ADDRESS": {},
	"DESKTOP_SESSION":          {},
	"DISPLAY":                  {},
	"HOME":                     {},
	"LANG":                     {},
	"LOGNAME":                  {},
	"USER":                     {},
	"WAYLAND_DISPLAY":          {},
	"XDG_CURRENT_DESKTOP":      {},
	"XDG_DATA_DIRS":            {},
	"XDG_RUNTIME_DIR":          {},
	"XDG_SESSION_TYPE":         {},
}

func BuildEnvironment(base map[string]string) []string {
	values := map[string]string{"PATH": DefaultPath}
	for key, value := range base {
		if _, ok := allowedExactEnvironment[key]; ok || strings.HasPrefix(key, "LC_") {
			values[key] = value
		}
	}

	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	environment := make([]string, 0, len(keys))
	for _, key := range keys {
		environment = append(environment, key+"="+values[key])
	}
	return environment
}

func ResolveWorkingDirectory(requested string, home string) (string, error) {
	workingDirectory := strings.TrimSpace(requested)
	if workingDirectory == "" {
		workingDirectory = strings.TrimSpace(home)
	}
	if workingDirectory == "" {
		return "", fmt.Errorf("%w: missing working directory", ErrInvalidRequest)
	}
	if !filepath.IsAbs(workingDirectory) {
		return "", fmt.Errorf("%w: working directory must be absolute", ErrInvalidRequest)
	}
	info, err := os.Stat(workingDirectory)
	if err != nil {
		return "", fmt.Errorf("%w: working directory unavailable: %v", ErrInvalidRequest, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%w: working directory is not a directory", ErrInvalidRequest)
	}
	return workingDirectory, nil
}
