package compositorbridge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadLayoutSettingsUsesDefaultsWhenMissing(t *testing.T) {
	settings, err := LoadLayoutSettings(filepath.Join(t.TempDir(), "missing.json"))
	if err != nil {
		t.Fatal(err)
	}
	if settings.Rule != LayoutRuleMasterStack || settings.Mode != LayoutModeZones || settings.MasterCount != 1 || settings.MasterRatio != 0.5 || !settings.SmartGaps {
		t.Fatalf("settings = %+v", settings)
	}
}

func TestLoadLayoutSettingsRejectsInvalidValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "layout-settings.json")
	if err := os.WriteFile(path, []byte(`{"rule":"master_stack","mode":"zones","master_count":0,"master_ratio":0.5}`), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := LoadLayoutSettings(path)
	if err == nil || !strings.Contains(err.Error(), "master_count must be positive") {
		t.Fatalf("err = %v, want master_count validation", err)
	}
}

func TestLayoutModePersistsAndLoadsAfterRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agora-de", "layout-settings.json")
	bridge := New(Config{LayoutSettingsPath: path})
	response, err := bridge.SetLayoutMode(SetLayoutModeRequest{Mode: LayoutModeFreeform})
	if err != nil {
		t.Fatal(err)
	}
	if response.Layout == nil || response.Layout.Settings.Mode != LayoutModeFreeform {
		t.Fatalf("response = %+v", response)
	}

	restarted := New(Config{LayoutSettingsPath: path})
	layout := restarted.GetLayout().Layout
	if layout.Mode != LayoutModeFreeform || layout.Settings.Mode != LayoutModeFreeform {
		t.Fatalf("layout after restart = %+v", layout)
	}
}
