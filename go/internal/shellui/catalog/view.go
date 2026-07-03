package catalog

import "agora-de.local/go/internal/appcatalog"

type AppView struct {
	ID   string
	Name string
	Icon string
}

func VisibleAppViews(source *appcatalog.Catalog) []AppView {
	entries := source.VisibleEntries()
	views := make([]AppView, 0, len(entries))
	for _, entry := range entries {
		views = append(views, AppView{
			ID:   entry.ID,
			Name: entry.Name,
			Icon: entry.Icon,
		})
	}
	return views
}

