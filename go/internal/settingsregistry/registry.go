package settingsregistry

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"agora-de.local/go/internal/settingsprotocol"
)

const DefaultAdapterTimeout = 2 * time.Second

// Module is the deliberately small first-party registration seam. The common
// registry knows only generated manifest/availability contracts and delegates
// module-specific HTTP payloads to Handler.
type Module interface {
	Manifest() settingsprotocol.SettingsModuleManifest
	Availability(context.Context) settingsprotocol.SettingsModuleAvailability
	Handler() http.Handler
}

type Registry struct {
	modules []Module
	byID    map[string]Module
	timeout time.Duration
}

func New(modules []Module, timeout time.Duration) (*Registry, error) {
	if timeout <= 0 {
		timeout = DefaultAdapterTimeout
	}
	registry := &Registry{
		modules: append([]Module(nil), modules...),
		byID:    make(map[string]Module, len(modules)),
		timeout: timeout,
	}
	for _, module := range modules {
		if module == nil {
			return nil, fmt.Errorf("settings module is nil")
		}
		manifest := module.Manifest()
		if err := validateManifest(manifest); err != nil {
			return nil, fmt.Errorf("settings module %q: %w", manifest.ID, err)
		}
		if _, exists := registry.byID[manifest.ID]; exists {
			return nil, fmt.Errorf("duplicate settings module id %q", manifest.ID)
		}
		registry.byID[manifest.ID] = module
	}
	return registry, nil
}

func (registry *Registry) Module(id string) (Module, bool) {
	module, ok := registry.byID[id]
	return module, ok
}

func (registry *Registry) Timeout() time.Duration {
	return registry.timeout
}

func (registry *Registry) Catalog(ctx context.Context) settingsprotocol.SettingsCatalogResponse {
	entries := make([]settingsprotocol.SettingsCatalogEntry, len(registry.modules))
	var wait sync.WaitGroup
	wait.Add(len(registry.modules))
	for index, module := range registry.modules {
		go func(index int, module Module) {
			defer wait.Done()
			manifest := module.Manifest()
			moduleContext, cancel := context.WithTimeout(ctx, registry.timeout)
			defer cancel()
			result := make(chan settingsprotocol.SettingsModuleAvailability, 1)
			go func() {
				result <- module.Availability(moduleContext)
			}()
			availability := settingsprotocol.SettingsModuleAvailability{
				State:  settingsprotocol.SettingsAvailabilityUnavailable,
				Reason: "adapter timed out",
			}
			select {
			case availability = <-result:
			case <-moduleContext.Done():
			}
			entries[index] = settingsprotocol.SettingsCatalogEntry{
				Manifest:     manifest,
				Availability: availability,
			}
		}(index, module)
	}
	wait.Wait()
	return settingsprotocol.SettingsCatalogResponse{
		SchemaVersion: settingsprotocol.SettingsSchemaVersion,
		Modules:       entries,
	}
}

func validateManifest(manifest settingsprotocol.SettingsModuleManifest) error {
	if !stableID(manifest.ID) || !stableID(manifest.Route) {
		return fmt.Errorf("id and route must be stable lowercase identifiers")
	}
	if manifest.Title == "" || manifest.Summary == "" || manifest.ContractVersion == 0 {
		return fmt.Errorf("title, summary, and non-zero contract version are required")
	}
	if manifest.BackendAdapter == "" || manifest.UIEntryPoint == "" || manifest.Icon == "" {
		return fmt.Errorf("adapter, UI entry point, and icon are required")
	}
	if len(manifest.SearchTerms) > 24 {
		return fmt.Errorf("search terms exceed contract bound")
	}
	capabilities := append([]settingsprotocol.SettingsCapability(nil), manifest.Capabilities...)
	sort.Slice(capabilities, func(left, right int) bool { return capabilities[left] < capabilities[right] })
	if !containsCapability(capabilities, settingsprotocol.SettingsCapabilityLoad) {
		return fmt.Errorf("load capability is required")
	}
	return nil
}

func stableID(value string) bool {
	if value == "" || len(value) > 64 || strings.ToLower(value) != value {
		return false
	}
	for _, character := range value {
		if (character < 'a' || character > 'z') && (character < '0' || character > '9') && character != '-' {
			return false
		}
	}
	return true
}

func containsCapability(capabilities []settingsprotocol.SettingsCapability, target settingsprotocol.SettingsCapability) bool {
	for _, capability := range capabilities {
		if capability == target {
			return true
		}
	}
	return false
}
