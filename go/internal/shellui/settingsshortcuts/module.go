package settingsshortcuts

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"agora-de.local/go/internal/settingsprotocol"
)

const beginMarker = "# >>> agora-de-keybindings (managed - do not edit; regenerate via generate-wayfire-keybindings.py)"
const endMarker = "# <<< agora-de-keybindings"

var acceleratorPattern = regexp.MustCompile(`^(?:<(?:super|ctrl|alt|shift)> )+KEY_[A-Z0-9_]+$`)
var idPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,47}$`)

type Config struct{ KeymapPath, WayfireConfigPath string }
type Module struct{ keymapPath, wayfirePath string }
type binding struct{ id, keys, command string }

var allowedCommands = map[string]string{
	"settings": "launch --arg agora-de-shell-settings", "focus_next": "surface focus-next", "focus_prev": "surface focus-prev", "move_left": "surface move --surface focused --direction left", "move_right": "surface move --surface focused --direction right", "move_up": "surface move --surface focused --direction up", "move_down": "surface move --surface focused --direction down", "swap_master": "surface swap-master --surface focused", "promote": "surface promote --surface focused", "toggle_float": "surface set-floating --surface focused", "cycle_mode": "layout cycle-mode", "cycle_rule": "layout cycle-rule", "close_focused": "surface close --surface focused",
}
var titles = map[string]string{"settings": "Open Settings", "focus_next": "Focus next window", "focus_prev": "Focus previous window", "move_left": "Move window left", "move_right": "Move window right", "move_up": "Move window up", "move_down": "Move window down", "swap_master": "Swap with master", "promote": "Promote to master", "toggle_float": "Toggle floating", "cycle_mode": "Cycle layout mode", "cycle_rule": "Cycle layout rule", "close_focused": "Close focused window"}
var defaultKeys = map[string]string{"settings": "<super> KEY_COMMA", "focus_next": "<super> KEY_J", "focus_prev": "<super> KEY_K", "move_left": "<super> <shift> KEY_H", "move_right": "<super> <shift> KEY_L", "move_up": "<super> <shift> KEY_K", "move_down": "<super> <shift> KEY_J", "swap_master": "<super> <shift> KEY_ENTER", "promote": "<super> <shift> KEY_M", "toggle_float": "<super> <shift> KEY_F", "cycle_mode": "<super> <shift> KEY_SPACE", "cycle_rule": "<super> <shift> KEY_R", "close_focused": "<super> <shift> KEY_Q"}

func New(config Config) *Module {
	return &Module{keymapPath: config.KeymapPath, wayfirePath: config.WayfireConfigPath}
}
func (module *Module) Manifest() settingsprotocol.SettingsModuleManifest {
	return settingsprotocol.SettingsModuleManifest{ID: settingsprotocol.ShortcutsModuleID, Category: settingsprotocol.SettingsCategoryPersonal, Title: "Shortcuts", Summary: "Review and edit managed Agora keyboard shortcuts.", Icon: "shortcuts", Route: settingsprotocol.ShortcutsModuleID, SearchTerms: []string{"keyboard", "keys", "bindings", "hotkeys"}, Capabilities: []settingsprotocol.SettingsCapability{settingsprotocol.SettingsCapabilityLoad, settingsprotocol.SettingsCapabilityValidate, settingsprotocol.SettingsCapabilityApply, settingsprotocol.SettingsCapabilityRestoreDefaults}, BackendAdapter: "settings-shortcuts", UIEntryPoint: "settings-shortcuts", ContractVersion: settingsprotocol.ShortcutsContractVersion}
}
func (module *Module) Availability(context.Context) settingsprotocol.SettingsModuleAvailability {
	if _, err := module.snapshot(); err != nil {
		return settingsprotocol.SettingsModuleAvailability{State: settingsprotocol.SettingsAvailabilityUnavailable, Reason: err.Error()}
	}
	return settingsprotocol.SettingsModuleAvailability{State: settingsprotocol.SettingsAvailabilityAvailable}
}
func (module *Module) Handler() http.Handler { return http.HandlerFunc(module.serveHTTP) }

func (module *Module) serveHTTP(response http.ResponseWriter, request *http.Request) {
	switch request.URL.Path {
	case "/state":
		if request.Method != http.MethodGet {
			methodNotAllowed(response, http.MethodGet)
			return
		}
		state, err := module.snapshot()
		if err != nil {
			unavailable(response, err)
			return
		}
		writeJSON(response, http.StatusOK, state)
	case "/validate":
		if request.Method != http.MethodPost {
			methodNotAllowed(response, http.MethodPost)
			return
		}
		var body settingsprotocol.ShortcutValidateRequest
		if err := decodeStrict(request, &body); err != nil {
			invalid(response, err.Error())
			return
		}
		module.validate(response, body)
	case "/apply":
		if request.Method != http.MethodPost {
			methodNotAllowed(response, http.MethodPost)
			return
		}
		var body settingsprotocol.ShortcutApplyRequest
		if err := decodeStrict(request, &body); err != nil {
			invalid(response, err.Error())
			return
		}
		module.apply(response, body)
	case "/defaults":
		if request.Method != http.MethodPost {
			methodNotAllowed(response, http.MethodPost)
			return
		}
		var body settingsprotocol.SettingsDefaultsRequest
		if err := decodeStrict(request, &body); err != nil {
			invalid(response, err.Error())
			return
		}
		state, err := module.snapshot()
		if err != nil {
			unavailable(response, err)
			return
		}
		if body.ContractVersion != settingsprotocol.ShortcutsContractVersion {
			invalid(response, "unsupported Shortcuts contract version")
			return
		}
		if body.BaseRevision != state.Revision {
			stale(response)
			return
		}
		writeJSON(response, http.StatusOK, state.Defaults)
	default:
		http.NotFound(response, request)
	}
}
func (module *Module) validate(response http.ResponseWriter, body settingsprotocol.ShortcutValidateRequest) {
	state, err := module.snapshot()
	if err != nil {
		unavailable(response, err)
		return
	}
	if body.BaseRevision != state.Revision {
		stale(response)
		return
	}
	issues := validateKeymap(body.ContractVersion, body.Draft, state.Definitions)
	writeJSON(response, http.StatusOK, settingsprotocol.SettingsValidationResponse{Valid: len(issues) == 0, Issues: issues})
}
func (module *Module) apply(response http.ResponseWriter, body settingsprotocol.ShortcutApplyRequest) {
	state, err := module.snapshot()
	if err != nil {
		unavailable(response, err)
		return
	}
	if body.BaseRevision != state.Revision {
		stale(response)
		return
	}
	if issues := validateKeymap(body.ContractVersion, body.Draft, state.Definitions); len(issues) > 0 {
		writeJSON(response, http.StatusBadRequest, settingsprotocol.SettingsError{Code: settingsprotocol.SettingsErrorValidation, Message: "Shortcut keymap is invalid", Issues: issues})
		return
	}
	if err := module.publish(body.Draft); err != nil {
		writeJSON(response, http.StatusServiceUnavailable, settingsprotocol.SettingsError{Code: settingsprotocol.SettingsErrorApplyFailed, Message: err.Error(), Retryable: true})
		return
	}
	next, err := module.snapshot()
	if err != nil {
		unavailable(response, err)
		return
	}
	writeJSON(response, http.StatusOK, settingsprotocol.ShortcutApplyResponse{State: next, Outcome: settingsprotocol.SettingsApplyOutcome{Kind: settingsprotocol.SettingsApplyApplied}})
}

func (module *Module) snapshot() (settingsprotocol.ShortcutSettingsState, error) {
	data, err := os.ReadFile(module.keymapPath)
	if err != nil {
		return settingsprotocol.ShortcutSettingsState{}, fmt.Errorf("read managed keymap: %w", err)
	}
	bindings, _, err := parseKeymap(string(data))
	if err != nil {
		return settingsprotocol.ShortcutSettingsState{}, err
	}
	assignments := make([]settingsprotocol.ShortcutAssignment, 0, len(bindings))
	defaults := make([]settingsprotocol.ShortcutAssignment, 0, len(bindings))
	definitions := make([]settingsprotocol.ShortcutDefinition, 0, len(bindings))
	for _, item := range bindings {
		assignments = append(assignments, settingsprotocol.ShortcutAssignment{ID: item.id, Accelerator: item.keys})
		defaults = append(defaults, settingsprotocol.ShortcutAssignment{ID: item.id, Accelerator: defaultKeys[item.id]})
		definitions = append(definitions, settingsprotocol.ShortcutDefinition{ID: item.id, Title: titles[item.id], Group: group(item.id), Reserved: item.id == "settings"})
	}
	return settingsprotocol.ShortcutSettingsState{ModuleID: settingsprotocol.ShortcutsModuleID, ContractVersion: settingsprotocol.ShortcutsContractVersion, Revision: revision(string(data)), Active: settingsprotocol.ShortcutKeymap{Assignments: assignments}, Defaults: settingsprotocol.ShortcutKeymap{Assignments: defaults}, Definitions: definitions, Availability: settingsprotocol.SettingsModuleAvailability{State: settingsprotocol.SettingsAvailabilityAvailable}}, nil
}

func parseKeymap(text string) ([]binding, string, error) {
	lines := strings.Split(text, "\n")
	compositorctl := "agora-de-compositorctl"
	var current *binding
	items := []binding{}
	flush := func() error {
		if current == nil {
			return nil
		}
		expected, ok := allowedCommands[current.id]
		if !ok || current.command != expected {
			return fmt.Errorf("managed binding %q has an unapproved command", current.id)
		}
		if !acceleratorPattern.MatchString(current.keys) {
			return fmt.Errorf("managed binding %q has invalid keys", current.id)
		}
		items = append(items, *current)
		current = nil
		return nil
	}
	for _, line := range lines {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "compositorctl = ") {
			compositorctl = strings.Trim(strings.TrimSpace(strings.TrimPrefix(trim, "compositorctl = ")), "\"")
		}
		if trim == "[[binding]]" {
			if err := flush(); err != nil {
				return nil, "", err
			}
			current = &binding{}
			continue
		}
		if current == nil {
			continue
		}
		if strings.HasPrefix(trim, "name = ") {
			current.id = strings.Trim(strings.TrimSpace(strings.TrimPrefix(trim, "name = ")), "\"")
		}
		if strings.HasPrefix(trim, "keys = ") {
			current.keys = strings.Trim(strings.TrimSpace(strings.TrimPrefix(trim, "keys = ")), "\"")
		}
		if strings.HasPrefix(trim, "command = ") {
			current.command = strings.Trim(strings.TrimSpace(strings.TrimPrefix(trim, "command = ")), "\"")
		}
	}
	if err := flush(); err != nil {
		return nil, "", err
	}
	if len(items) == 0 {
		return nil, "", errors.New("managed keymap has no bindings")
	}
	return items, compositorctl, nil
}

func validateKeymap(contract uint16, draft settingsprotocol.ShortcutKeymap, definitions []settingsprotocol.ShortcutDefinition) []settingsprotocol.SettingsValidationIssue {
	issues := []settingsprotocol.SettingsValidationIssue{}
	if contract != settingsprotocol.ShortcutsContractVersion {
		issues = append(issues, issue("contractVersion", "unsupported_contract_version", "Shortcuts contract version is unsupported."))
	}
	known := map[string]bool{}
	reserved := map[string]bool{}
	for _, definition := range definitions {
		known[definition.ID] = true
		reserved[definition.ID] = definition.Reserved
	}
	seenIDs := map[string]bool{}
	seenKeys := map[string]string{}
	for index, item := range draft.Assignments {
		field := fmt.Sprintf("assignments.%d", index)
		if !idPattern.MatchString(item.ID) || !known[item.ID] {
			issues = append(issues, issue(field+".id", "unknown_binding", "Only managed binding IDs may be edited."))
		}
		if seenIDs[item.ID] {
			issues = append(issues, issue(field+".id", "duplicate_binding", "Each binding must appear once."))
		}
		seenIDs[item.ID] = true
		if !acceleratorPattern.MatchString(item.Accelerator) {
			issues = append(issues, issue(field+".accelerator", "invalid_accelerator", "Use modifiers followed by one KEY_ token."))
		}
		normalized := strings.ToUpper(item.Accelerator)
		if other := seenKeys[normalized]; other != "" {
			issues = append(issues, issue(field+".accelerator", "conflict", "Shortcut conflicts with "+other+"."))
		}
		seenKeys[normalized] = item.ID
	}
	for id := range known {
		if !seenIDs[id] {
			code := "missing_binding"
			message := "Managed bindings cannot be removed."
			if reserved[id] {
				code = "reserved_binding"
				message = "The Settings launch shortcut must remain assigned."
			}
			issues = append(issues, issue("assignments", code, message))
		}
	}
	return issues
}

func (module *Module) publish(draft settingsprotocol.ShortcutKeymap) error {
	source, err := os.ReadFile(module.keymapPath)
	if err != nil {
		return err
	}
	wayfire, err := os.ReadFile(module.wayfirePath)
	if err != nil {
		return err
	}
	bindings, ctl, err := parseKeymap(string(source))
	if err != nil {
		return err
	}
	updates := map[string]string{}
	for _, item := range draft.Assignments {
		updates[item.ID] = item.Accelerator
	}
	newSource := rewriteKeys(string(source), updates)
	for index := range bindings {
		bindings[index].keys = updates[bindings[index].id]
	}
	block := renderBlock(module.keymapPath, ctl, bindings)
	newWayfire, err := splice(string(wayfire), block)
	if err != nil {
		return err
	}
	if err := atomicWrite(module.keymapPath, []byte(newSource), 0o600); err != nil {
		return err
	}
	if err := atomicWrite(module.wayfirePath, []byte(newWayfire), 0o600); err != nil {
		_ = atomicWrite(module.keymapPath, source, 0o600)
		return fmt.Errorf("publish Wayfire keymap: %w", err)
	}
	return nil
}
func rewriteKeys(text string, updates map[string]string) string {
	lines := strings.Split(text, "\n")
	id := ""
	for index, line := range lines {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "name = ") {
			id = strings.Trim(strings.TrimSpace(strings.TrimPrefix(trim, "name = ")), "\"")
		}
		if strings.HasPrefix(trim, "keys = ") && updates[id] != "" {
			prefix := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
			lines[index] = prefix + "keys = \"" + updates[id] + "\""
		}
	}
	return strings.Join(lines, "\n")
}
func renderBlock(path, ctl string, bindings []binding) string {
	var out strings.Builder
	out.WriteString(beginMarker + "\n# source: " + path + "\n")
	for _, item := range bindings {
		fmt.Fprintf(&out, "binding_%s = %s\ncommand_%s = %s %s\n", item.id, item.keys, item.id, ctl, item.command)
	}
	out.WriteString(endMarker + "\n")
	return out.String()
}
func splice(text, block string) (string, error) {
	if strings.Contains(text, beginMarker) != strings.Contains(text, endMarker) {
		return "", errors.New("Wayfire managed keymap markers are unbalanced")
	}
	if start := strings.Index(text, beginMarker); start >= 0 {
		end := strings.Index(text[start:], endMarker)
		end = start + end + len(endMarker)
		if end < len(text) && text[end] == '\n' {
			end++
		}
		return text[:start] + block + text[end:], nil
	}
	marker := "[command]\n"
	index := strings.Index(text, marker)
	if index < 0 {
		return strings.TrimRight(text, "\n") + "\n\n[command]\n" + block, nil
	}
	index += len(marker)
	return text[:index] + block + text[index:], nil
}
func atomicWrite(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	temp, err := os.CreateTemp(dir, ".shortcuts-*")
	if err != nil {
		return err
	}
	name := temp.Name()
	defer os.Remove(name)
	if err := temp.Chmod(mode); err != nil {
		temp.Close()
		return err
	}
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}
func group(id string) string {
	if strings.HasPrefix(id, "move_") || id == "promote" || id == "swap_master" || id == "toggle_float" || id == "close_focused" {
		return "Windows"
	}
	if strings.HasPrefix(id, "cycle_") {
		return "Layout"
	}
	return "Navigation"
}
func revision(value string) uint64 {
	hash := fnv.New64a()
	_, _ = hash.Write([]byte(value))
	return hash.Sum64()
}
func issue(field, code, message string) settingsprotocol.SettingsValidationIssue {
	return settingsprotocol.SettingsValidationIssue{Field: field, Code: code, Message: message}
}
func stale(response http.ResponseWriter) {
	writeJSON(response, http.StatusConflict, settingsprotocol.SettingsError{Code: settingsprotocol.SettingsErrorStaleRevision, Message: "shortcut keymap changed; reload before applying", Retryable: true})
}
func unavailable(response http.ResponseWriter, err error) {
	writeJSON(response, http.StatusServiceUnavailable, settingsprotocol.SettingsError{Code: settingsprotocol.SettingsErrorUnavailable, Message: err.Error(), Retryable: true})
}
func invalid(response http.ResponseWriter, message string) {
	writeJSON(response, http.StatusBadRequest, settingsprotocol.SettingsError{Code: settingsprotocol.SettingsErrorInvalidRequest, Message: message})
}
func methodNotAllowed(response http.ResponseWriter, method string) {
	response.Header().Set("Allow", method)
	invalid(response, "method not allowed")
}
func decodeStrict(request *http.Request, target any) error {
	decoder := json.NewDecoder(io.LimitReader(request.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("request must contain one JSON value")
	}
	return nil
}
func writeJSON(response http.ResponseWriter, status int, payload any) {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(payload)
}

func SortedAssignments(keymap settingsprotocol.ShortcutKeymap) []settingsprotocol.ShortcutAssignment {
	result := append([]settingsprotocol.ShortcutAssignment(nil), keymap.Assignments...)
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}
