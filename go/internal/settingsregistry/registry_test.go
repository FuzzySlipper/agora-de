package settingsregistry

import (
	"context"
	"net/http"
	"testing"
	"time"

	"agora-de.local/go/internal/settingsprotocol"
)

type fixtureModule struct {
	id           string
	availability settingsprotocol.SettingsModuleAvailability
	delay        time.Duration
}

func (module fixtureModule) Manifest() settingsprotocol.SettingsModuleManifest {
	return settingsprotocol.SettingsModuleManifest{
		ID:              module.id,
		Category:        settingsprotocol.SettingsCategorySystem,
		Title:           module.id,
		Summary:         "fixture settings module",
		Icon:            "settings",
		Route:           module.id,
		SearchTerms:     []string{"fixture"},
		Capabilities:    []settingsprotocol.SettingsCapability{settingsprotocol.SettingsCapabilityLoad},
		BackendAdapter:  module.id,
		UIEntryPoint:    module.id,
		ContractVersion: 1,
	}
}

func (module fixtureModule) Availability(ctx context.Context) settingsprotocol.SettingsModuleAvailability {
	if module.delay > 0 {
		select {
		case <-time.After(module.delay):
		case <-ctx.Done():
			return settingsprotocol.SettingsModuleAvailability{
				State:  settingsprotocol.SettingsAvailabilityUnavailable,
				Reason: "cancelled",
			}
		}
	}
	return module.availability
}

func (fixtureModule) Handler() http.Handler { return http.NotFoundHandler() }

func TestCatalogIsolatesSlowAndUnavailableModules(t *testing.T) {
	registry, err := New([]Module{
		fixtureModule{id: "healthy", availability: settingsprotocol.SettingsModuleAvailability{State: settingsprotocol.SettingsAvailabilityAvailable}},
		fixtureModule{id: "offline", availability: settingsprotocol.SettingsModuleAvailability{State: settingsprotocol.SettingsAvailabilityUnavailable, Reason: "service absent"}},
		fixtureModule{id: "slow", delay: time.Second, availability: settingsprotocol.SettingsModuleAvailability{State: settingsprotocol.SettingsAvailabilityAvailable}},
	}, 20*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}

	started := time.Now()
	catalog := registry.Catalog(context.Background())
	if elapsed := time.Since(started); elapsed > 250*time.Millisecond {
		t.Fatalf("catalog waited for slow module: %s", elapsed)
	}
	if len(catalog.Modules) != 3 {
		t.Fatalf("catalog modules = %d, want 3", len(catalog.Modules))
	}
	if catalog.Modules[0].Availability.State != settingsprotocol.SettingsAvailabilityAvailable {
		t.Fatalf("healthy module = %+v", catalog.Modules[0])
	}
	if catalog.Modules[1].Availability.Reason != "service absent" {
		t.Fatalf("offline module = %+v", catalog.Modules[1])
	}
	if catalog.Modules[2].Availability.State != settingsprotocol.SettingsAvailabilityUnavailable {
		t.Fatalf("slow module = %+v", catalog.Modules[2])
	}
}

func TestRegistrationRejectsInvalidAndDuplicateManifests(t *testing.T) {
	if _, err := New([]Module{fixtureModule{id: "Bad ID"}}, time.Second); err == nil {
		t.Fatal("invalid manifest was accepted")
	}
	if _, err := New([]Module{fixtureModule{id: "same"}, fixtureModule{id: "same"}}, time.Second); err == nil {
		t.Fatal("duplicate module id was accepted")
	}
}

func TestModuleLookupDoesNotRequireCentralPayloadSwitch(t *testing.T) {
	registry, err := New([]Module{fixtureModule{id: "fixture"}}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	module, ok := registry.Module("fixture")
	if !ok || module.Manifest().UIEntryPoint != "fixture" {
		t.Fatalf("fixture module lookup failed: %v %+v", ok, module)
	}
	if _, ok := registry.Module("unknown"); ok {
		t.Fatal("unknown module unexpectedly registered")
	}
}
