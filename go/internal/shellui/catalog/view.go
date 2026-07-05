package catalog

import (
	"path/filepath"
	"strings"

	"agora-de.local/go/internal/appcatalog"
)

type AppView struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Icon           string   `json:"icon"`
	IconKind       string   `json:"iconKind"`
	IconRef        string   `json:"iconRef"`
	IconLabel      string   `json:"iconLabel"`
	Category       string   `json:"category"`
	Categories     []string `json:"categories,omitempty"`
	Launchable     bool     `json:"launchable,omitempty"`
	DisabledReason string   `json:"disabledReason,omitempty"`
}

func VisibleAppViews(source *appcatalog.Catalog) []AppView {
	entries := source.VisibleEntries()
	views := make([]AppView, 0, len(entries))
	for _, entry := range entries {
		view := AppView{
			ID:         entry.ID,
			Name:       entry.Name,
			Icon:       entry.Icon,
			IconKind:   IconKind(entry.Icon),
			IconRef:    IconRef(entry.Icon),
			IconLabel:  IconLabel(entry),
			Category:   CategoryGroup(entry.Categories),
			Categories: append([]string(nil), entry.Categories...),
			Launchable: entry.Launchable(),
		}
		if !view.Launchable {
			view.DisabledReason = "unsupported desktop entry"
		}
		views = append(views, view)
	}
	return views
}

func IconLabel(entry appcatalog.Entry) string {
	name := strings.TrimSpace(entry.Name)
	if name == "" {
		name = strings.TrimSpace(entry.ID)
	}
	if name == "" {
		return "?"
	}
	for _, char := range name {
		return strings.ToUpper(string(char))
	}
	return "?"
}

func CategoryGroup(categories []string) string {
	categorySet := map[string]bool{}
	for _, category := range categories {
		categorySet[category] = true
	}
	for _, rule := range categoryRules {
		for _, category := range rule.Categories {
			if categorySet[category] {
				return rule.Label
			}
		}
	}
	return "Other"
}

func IconKind(icon string) string {
	icon = strings.TrimSpace(icon)
	switch {
	case icon == "":
		return "fallback"
	case filepath.IsAbs(icon):
		return "path"
	default:
		return "theme"
	}
}

func IconRef(icon string) string {
	icon = strings.TrimSpace(icon)
	if icon == "" {
		return "application-x-executable"
	}
	return icon
}

type categoryRule struct {
	Label      string
	Categories []string
}

var categoryRules = []categoryRule{
	{Label: "Internet", Categories: []string{"Network", "WebBrowser", "Email", "Chat", "InstantMessaging"}},
	{Label: "Development", Categories: []string{"Development", "IDE", "GUIDesigner"}},
	{Label: "Office", Categories: []string{"Office", "WordProcessor", "Spreadsheet", "Presentation"}},
	{Label: "Media", Categories: []string{"AudioVideo", "Audio", "Video", "Player", "Graphics", "Photography"}},
	{Label: "System", Categories: []string{"System", "Settings", "TerminalEmulator", "FileManager"}},
	{Label: "Utilities", Categories: []string{"Utility", "Accessories", "Calculator", "TextEditor", "Archiving"}},
	{Label: "Games", Categories: []string{"Game"}},
}
