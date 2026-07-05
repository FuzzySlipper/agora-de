package nativelaunch

import (
	"fmt"
	"path/filepath"
	"strings"

	"agora-de.local/go/internal/appcatalog"
)

// BuildArgv turns a desktop-entry Exec value into an argv vector without shell
// evaluation. It intentionally supports only field codes that do not require
// selected files, URLs, or shell-owned context.
func BuildArgv(entry appcatalog.Entry, desktopFilePath string) ([]string, error) {
	execValue := strings.TrimSpace(entry.Exec)
	if execValue == "" {
		return nil, fmt.Errorf("%w: missing exec", ErrInvalidRequest)
	}

	var args []string
	var current strings.Builder
	inQuote := false
	escaped := false
	tokenStarted := false

	flush := func() {
		if tokenStarted {
			args = append(args, current.String())
			current.Reset()
			tokenStarted = false
		}
	}

	for index := 0; index < len(execValue); index++ {
		character := execValue[index]
		if escaped {
			current.WriteByte(character)
			escaped = false
			tokenStarted = true
			continue
		}

		switch character {
		case '\\':
			escaped = true
			tokenStarted = true
		case '"':
			inQuote = !inQuote
			tokenStarted = true
		case ' ', '\t', '\n', '\r':
			if inQuote {
				current.WriteByte(character)
				tokenStarted = true
				continue
			}
			flush()
		case '%':
			if index+1 >= len(execValue) {
				return nil, fmt.Errorf("%w: unterminated field code", ErrUnsupportedFieldCode)
			}
			index++
			wrote, err := appendFieldCode(&current, entry, desktopFilePath, execValue[index])
			if err != nil {
				return nil, err
			}
			tokenStarted = tokenStarted || wrote
		default:
			current.WriteByte(character)
			tokenStarted = true
		}
	}

	if escaped {
		return nil, fmt.Errorf("%w: unterminated escape", ErrInvalidRequest)
	}
	if inQuote {
		return nil, fmt.Errorf("%w: unterminated quote", ErrInvalidRequest)
	}
	flush()

	if len(args) == 0 || strings.TrimSpace(args[0]) == "" {
		return nil, fmt.Errorf("%w: missing executable", ErrInvalidRequest)
	}
	return applyLaunchHints(args), nil
}

func applyLaunchHints(args []string) []string {
	if len(args) == 0 || !isChromiumFamilyExecutable(args[0]) || hasOzonePlatformArg(args) {
		return args
	}
	result := append([]string(nil), args...)
	return append(result, "--ozone-platform=wayland")
}

func isChromiumFamilyExecutable(executable string) bool {
	name := filepath.Base(strings.TrimSpace(executable))
	switch name {
	case "brave", "brave-browser", "chromium", "chromium-browser", "google-chrome", "google-chrome-stable":
		return true
	default:
		return false
	}
}

func hasOzonePlatformArg(args []string) bool {
	for _, arg := range args[1:] {
		if arg == "--ozone-platform" || strings.HasPrefix(arg, "--ozone-platform=") ||
			arg == "--ozone-platform-hint" || strings.HasPrefix(arg, "--ozone-platform-hint=") {
			return true
		}
	}
	return false
}

func appendFieldCode(builder *strings.Builder, entry appcatalog.Entry, desktopFilePath string, code byte) (bool, error) {
	switch code {
	case '%':
		builder.WriteByte('%')
		return true, nil
	case 'c':
		builder.WriteString(entry.Name)
		return true, nil
	case 'k':
		if !filepath.IsAbs(desktopFilePath) {
			return false, fmt.Errorf("%w: %%k requires absolute desktop file path", ErrInvalidRequest)
		}
		builder.WriteString(desktopFilePath)
		return true, nil
	case 'i', 'f', 'F', 'u', 'U':
		return false, nil
	default:
		return false, fmt.Errorf("%w: %%%c", ErrUnsupportedFieldCode, code)
	}
}
