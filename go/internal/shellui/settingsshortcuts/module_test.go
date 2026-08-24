package settingsshortcuts

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agora-de.local/go/internal/settingsprotocol"
)

const fixture = `compositorctl = "/usr/local/bin/agora-de-compositorctl"
# keep this comment
[[binding]]
name = "settings"
keys = "<super> KEY_COMMA"
command = "launch --arg agora-de-shell-settings"
[[binding]]
name = "focus_next"
keys = "<super> KEY_J"
command = "surface focus-next"
`

func TestManagedKeymapApplyPreservesUnrelatedWayfireConfig(t *testing.T) {
	dir := t.TempDir()
	keymap := filepath.Join(dir, "keybindings.toml")
	wayfire := filepath.Join(dir, "wayfire.ini")
	if err := os.WriteFile(keymap, []byte(fixture), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(wayfire, []byte("# outside\n[command]\nbinding_terminal = <super> KEY_ENTER\ncommand_terminal = foot\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	module := New(Config{KeymapPath: keymap, WayfireConfigPath: wayfire})
	state, err := module.snapshot()
	if err != nil {
		t.Fatal(err)
	}
	wire, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	var browser struct {
		Revision float64 `json:"revision"`
	}
	if err := json.Unmarshal(wire, &browser); err != nil {
		t.Fatal(err)
	}
	if state.Revision > maxJavaScriptSafeInteger || uint64(browser.Revision) != state.Revision {
		t.Fatalf("revision %d is not browser-safe", state.Revision)
	}
	draft := state.Active
	draft.Assignments[1].Accelerator = "<super> KEY_N"
	if issues := validateKeymap(1, draft, state.Definitions); len(issues) != 0 {
		t.Fatalf("issues=%+v", issues)
	}
	if err := module.publish(draft); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(wayfire)
	text := string(got)
	for _, want := range []string{"# outside", "binding_terminal", "binding_focus_next = <super> KEY_N", beginMarker, endMarker} {
		if !strings.Contains(text, want) {
			t.Fatalf("wayfire missing %q: %s", want, text)
		}
	}
}

func TestShortcutValidationRejectsConflictsUnknownAndReservedRemoval(t *testing.T) {
	definitions := []settingsprotocol.ShortcutDefinition{{ID: "settings", Reserved: true}, {ID: "focus_next"}}
	draft := settingsprotocol.ShortcutKeymap{Assignments: []settingsprotocol.ShortcutAssignment{{ID: "focus_next", Accelerator: "<super> KEY_J"}, {ID: "unknown", Accelerator: "<super> KEY_J"}}}
	issues := validateKeymap(1, draft, definitions)
	if len(issues) < 3 {
		t.Fatalf("issues=%+v", issues)
	}
}

func TestParseRejectsCommandTampering(t *testing.T) {
	_, _, err := parseKeymap(strings.Replace(fixture, "surface focus-next", "sh -c bad", 1))
	if err == nil {
		t.Fatal("tampered command accepted")
	}
}
