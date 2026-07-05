package appcatalog

import (
	"sort"
	"strings"
)

type Catalog struct {
	entries map[string]Entry
}

func NewCatalog() *Catalog {
	return &Catalog{entries: map[string]Entry{}}
}

func (catalog *Catalog) Add(entry Entry) {
	if strings.TrimSpace(entry.Type) == "" {
		entry.Type = "Application"
	}
	if len(entry.ExecTokens) == 0 && strings.TrimSpace(entry.Exec) != "" {
		entry.ExecTokens, entry.ExecSupported = NormalizeExec(entry.Exec)
	}
	catalog.entries[entry.ID] = entry
}

func (catalog *Catalog) Get(id string) (Entry, bool) {
	entry, ok := catalog.entries[id]
	return entry, ok
}

func (catalog *Catalog) VisibleEntries() []Entry {
	entries := make([]Entry, 0, len(catalog.entries))
	for _, entry := range catalog.entries {
		if entry.Visible() {
			entries = append(entries, entry)
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Name == entries[j].Name {
			return entries[i].ID < entries[j].ID
		}
		return entries[i].Name < entries[j].Name
	})
	return entries
}
