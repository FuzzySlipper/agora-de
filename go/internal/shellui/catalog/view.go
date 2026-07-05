package catalog

import "agora-de.local/go/internal/appcatalog"

type AppView struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Icon           string `json:"icon"`
	Launchable     bool   `json:"launchable,omitempty"`
	DisabledReason string `json:"disabledReason,omitempty"`
}

func VisibleAppViews(source *appcatalog.Catalog) []AppView {
	entries := source.VisibleEntries()
	views := make([]AppView, 0, len(entries))
	for _, entry := range entries {
		view := AppView{
			ID:         entry.ID,
			Name:       entry.Name,
			Icon:       entry.Icon,
			Launchable: entry.Launchable(),
		}
		if !view.Launchable {
			view.DisabledReason = "unsupported desktop entry"
		}
		views = append(views, view)
	}
	return views
}
