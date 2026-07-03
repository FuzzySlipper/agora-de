package appcatalog

type Catalog struct {
	entries map[string]Entry
}

func NewCatalog() *Catalog {
	return &Catalog{entries: map[string]Entry{}}
}

func (catalog *Catalog) Add(entry Entry) {
	catalog.entries[entry.ID] = entry
}

func (catalog *Catalog) Get(id string) (Entry, bool) {
	entry, ok := catalog.entries[id]
	return entry, ok
}

func (catalog *Catalog) VisibleEntries() []Entry {
	entries := make([]Entry, 0, len(catalog.entries))
	for _, entry := range catalog.entries {
		if !entry.NoDisplay {
			entries = append(entries, entry)
		}
	}
	return entries
}

