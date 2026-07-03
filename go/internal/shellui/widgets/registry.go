package widgets

type Registry struct {
	manifests map[string]Manifest
}

func NewRegistry() *Registry {
	return &Registry{manifests: map[string]Manifest{}}
}

func (registry *Registry) Add(manifest Manifest) error {
	if err := ValidateManifest(manifest); err != nil {
		return err
	}
	registry.manifests[manifest.ID] = manifest
	return nil
}

func (registry *Registry) Get(id string) (Manifest, bool) {
	manifest, ok := registry.manifests[id]
	return manifest, ok
}

func (registry *Registry) List() []Manifest {
	manifests := make([]Manifest, 0, len(registry.manifests))
	for _, manifest := range registry.manifests {
		manifests = append(manifests, manifest)
	}
	return manifests
}

