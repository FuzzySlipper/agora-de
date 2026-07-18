package settingsappearance

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
	"strings"

	"agora-de.local/go/internal/settingsprotocol"
	"agora-de.local/go/internal/shellui/theme"
)

const selectedThemeFile = "appearance/theme-id"

type Config struct {
	StateDir      string
	ActiveThemeID string
}

type Module struct {
	stateDir      string
	activeThemeID string
}

func New(config Config) *Module {
	stateDir := strings.TrimSpace(config.StateDir)
	if stateDir == "" {
		stateDir = filepath.Join(".", "shell-state")
	}
	active := strings.TrimSpace(config.ActiveThemeID)
	if active == "" {
		active = theme.DefaultThemeID
	}
	return &Module{stateDir: stateDir, activeThemeID: active}
}

func PersistedThemeID(stateDir string) string {
	data, err := os.ReadFile(filepath.Join(stateDir, selectedThemeFile))
	if err != nil {
		return ""
	}
	id := strings.TrimSpace(string(data))
	if _, ok := theme.BuiltinManifest(id); !ok {
		return ""
	}
	return id
}

func (module *Module) Manifest() settingsprotocol.SettingsModuleManifest {
	return settingsprotocol.SettingsModuleManifest{ID: settingsprotocol.AppearanceModuleID, Category: settingsprotocol.SettingsCategoryPersonal, Title: "Appearance", Summary: "Preview and apply a bundled Agora theme.", Icon: "appearance", Route: settingsprotocol.AppearanceModuleID, SearchTerms: []string{"theme", "colors", "appearance", "style"}, Capabilities: []settingsprotocol.SettingsCapability{settingsprotocol.SettingsCapabilityLoad, settingsprotocol.SettingsCapabilityValidate, settingsprotocol.SettingsCapabilityApply, settingsprotocol.SettingsCapabilityRestoreDefaults}, BackendAdapter: "settings-appearance", UIEntryPoint: "settings-appearance", ContractVersion: settingsprotocol.AppearanceContractVersion}
}

func (module *Module) Availability(context.Context) settingsprotocol.SettingsModuleAvailability {
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
		writeJSON(response, http.StatusOK, module.snapshot())
	case "/validate":
		if request.Method != http.MethodPost {
			methodNotAllowed(response, http.MethodPost)
			return
		}
		var body settingsprotocol.AppearanceValidateRequest
		if err := decodeStrict(request, &body); err != nil {
			invalid(response, err.Error())
			return
		}
		state := module.snapshot()
		if body.BaseRevision != state.Revision {
			stale(response)
			return
		}
		issues := validate(body.ContractVersion, body.Draft)
		writeJSON(response, http.StatusOK, settingsprotocol.SettingsValidationResponse{Valid: len(issues) == 0, Issues: issues})
	case "/apply":
		if request.Method != http.MethodPost {
			methodNotAllowed(response, http.MethodPost)
			return
		}
		var body settingsprotocol.AppearanceApplyRequest
		if err := decodeStrict(request, &body); err != nil {
			invalid(response, err.Error())
			return
		}
		state := module.snapshot()
		if body.BaseRevision != state.Revision {
			stale(response)
			return
		}
		if issues := validate(body.ContractVersion, body.Draft); len(issues) != 0 {
			writeJSON(response, http.StatusBadRequest, settingsprotocol.SettingsError{Code: settingsprotocol.SettingsErrorValidation, Message: "Appearance settings are invalid", Issues: issues})
			return
		}
		if err := module.persist(body.Draft.ThemeID); err != nil {
			writeJSON(response, http.StatusServiceUnavailable, settingsprotocol.SettingsError{Code: settingsprotocol.SettingsErrorApplyFailed, Message: err.Error(), Retryable: true})
			return
		}
		state = module.snapshot()
		writeJSON(response, http.StatusOK, settingsprotocol.AppearanceApplyResponse{State: state, Outcome: settingsprotocol.SettingsApplyOutcome{Kind: settingsprotocol.SettingsApplyRestartRequired, RestartComponent: "shell-chrome"}})
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
		state := module.snapshot()
		if body.ContractVersion != settingsprotocol.AppearanceContractVersion {
			invalid(response, "unsupported Appearance contract version")
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

func (module *Module) snapshot() settingsprotocol.AppearanceSettingsState {
	selected := PersistedThemeID(module.stateDir)
	if selected == "" {
		selected = module.activeThemeID
	}
	themes := make([]settingsprotocol.AppearanceThemeSummary, 0, len(theme.BuiltinManifests()))
	for _, manifest := range theme.BuiltinManifests() {
		themes = append(themes, settingsprotocol.AppearanceThemeSummary{ID: manifest.ID, Name: manifest.Name, Tokens: manifest.Tokens})
	}
	return settingsprotocol.AppearanceSettingsState{ModuleID: settingsprotocol.AppearanceModuleID, ContractVersion: settingsprotocol.AppearanceContractVersion, Revision: revision(selected), Active: settingsprotocol.AppearanceSettings{ThemeID: selected}, Defaults: settingsprotocol.AppearanceSettings{ThemeID: theme.DefaultThemeID}, Themes: themes, RestartRequired: selected != module.activeThemeID, Availability: settingsprotocol.SettingsModuleAvailability{State: settingsprotocol.SettingsAvailabilityAvailable}}
}

func validate(contract uint16, draft settingsprotocol.AppearanceSettings) []settingsprotocol.SettingsValidationIssue {
	issues := []settingsprotocol.SettingsValidationIssue{}
	if contract != settingsprotocol.AppearanceContractVersion {
		issues = append(issues, issue("contractVersion", "unsupported_contract_version", "Appearance contract version is unsupported."))
	}
	if _, ok := theme.BuiltinManifest(draft.ThemeID); !ok {
		issues = append(issues, issue("themeId", "unknown_theme", "Choose a validated bundled theme."))
	}
	return issues
}

func (module *Module) persist(id string) error {
	dir := filepath.Join(module.stateDir, "appearance")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create appearance state: %w", err)
	}
	temp, err := os.CreateTemp(dir, ".theme-id-*")
	if err != nil {
		return fmt.Errorf("create theme setting: %w", err)
	}
	name := temp.Name()
	defer os.Remove(name)
	if err := temp.Chmod(0o600); err != nil {
		temp.Close()
		return err
	}
	if _, err := temp.WriteString(id + "\n"); err != nil {
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
	if err := os.Rename(name, filepath.Join(dir, "theme-id")); err != nil {
		return fmt.Errorf("publish theme setting: %w", err)
	}
	return nil
}

func revision(id string) uint64 {
	hash := fnv.New64a()
	_, _ = hash.Write([]byte(id))
	return hash.Sum64()
}
func issue(field, code, message string) settingsprotocol.SettingsValidationIssue {
	return settingsprotocol.SettingsValidationIssue{Field: field, Code: code, Message: message}
}
func stale(response http.ResponseWriter) {
	writeJSON(response, http.StatusConflict, settingsprotocol.SettingsError{Code: settingsprotocol.SettingsErrorStaleRevision, Message: "appearance state changed; reload before applying", Retryable: true})
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
