package settingsdiagnostics

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"sync"
	"time"

	"agora-de.local/go/internal/settingsprotocol"
)

const overlayUnit = "agora-de-shell-overlay.service"
const productVersion = "settings-v1"

type Config struct {
	SystemctlPath        string
	DisplayAuthorityPath string
}

type Module struct {
	systemctl        string
	displayAuthority string
	mu               sync.Mutex
	revision         uint64
	fingerprint      string
	initialized      bool
}

func New(config Config) *Module {
	systemctl := strings.TrimSpace(config.SystemctlPath)
	if systemctl == "" {
		systemctl = "systemctl"
	}
	return &Module{systemctl: systemctl, displayAuthority: strings.TrimSpace(config.DisplayAuthorityPath)}
}

func (module *Module) Manifest() settingsprotocol.SettingsModuleManifest {
	return settingsprotocol.SettingsModuleManifest{
		ID:          settingsprotocol.DiagnosticsModuleID,
		Category:    settingsprotocol.SettingsCategorySystem,
		Title:       "Diagnostics & About",
		Summary:     "Inspect Agora services, versions, and diagnostic tools.",
		Icon:        "diagnostics",
		Route:       settingsprotocol.DiagnosticsModuleID,
		SearchTerms: []string{"overlay", "health", "service", "version", "support"},
		Capabilities: []settingsprotocol.SettingsCapability{
			settingsprotocol.SettingsCapabilityLoad,
			settingsprotocol.SettingsCapabilityValidate,
			settingsprotocol.SettingsCapabilityApply,
			settingsprotocol.SettingsCapabilityRestoreDefaults,
		},
		BackendAdapter:  "settings-diagnostics",
		UIEntryPoint:    "settings-diagnostics",
		ContractVersion: settingsprotocol.DiagnosticsContractVersion,
	}
}

func (module *Module) Availability(ctx context.Context) settingsprotocol.SettingsModuleAvailability {
	return module.snapshot(ctx).Availability
}

func (module *Module) Handler() http.Handler {
	return http.HandlerFunc(module.serveHTTP)
}

func (module *Module) serveHTTP(response http.ResponseWriter, request *http.Request) {
	switch request.URL.Path {
	case "/state":
		if request.Method != http.MethodGet {
			methodNotAllowed(response, http.MethodGet)
			return
		}
		state := module.snapshot(request.Context())
		if state.Availability.State != settingsprotocol.SettingsAvailabilityAvailable {
			writeError(response, http.StatusServiceUnavailable, settingsprotocol.SettingsError{
				Code:      settingsprotocol.SettingsErrorUnavailable,
				Message:   state.Availability.Reason,
				Retryable: true,
			})
			return
		}
		writeJSON(response, http.StatusOK, state)
	case "/validate":
		if request.Method != http.MethodPost {
			methodNotAllowed(response, http.MethodPost)
			return
		}
		var validate settingsprotocol.DiagnosticsValidateRequest
		if err := decodeStrict(request, &validate); err != nil {
			invalidRequest(response, err)
			return
		}
		module.validate(response, request.Context(), validate)
	case "/apply":
		if request.Method != http.MethodPost {
			methodNotAllowed(response, http.MethodPost)
			return
		}
		var apply settingsprotocol.DiagnosticsApplyRequest
		if err := decodeStrict(request, &apply); err != nil {
			invalidRequest(response, err)
			return
		}
		module.apply(response, request.Context(), apply)
	case "/defaults":
		if request.Method != http.MethodPost {
			methodNotAllowed(response, http.MethodPost)
			return
		}
		var defaults settingsprotocol.SettingsDefaultsRequest
		if err := decodeStrict(request, &defaults); err != nil {
			invalidRequest(response, err)
			return
		}
		module.defaults(response, request.Context(), defaults)
	case "/reset":
		if request.Method != http.MethodPost {
			methodNotAllowed(response, http.MethodPost)
			return
		}
		var reset settingsprotocol.SettingsResetRequest
		if err := decodeStrict(request, &reset); err != nil {
			invalidRequest(response, err)
			return
		}
		if reset.ContractVersion != settingsprotocol.DiagnosticsContractVersion {
			contractVersionError(response)
			return
		}
		writeJSON(response, http.StatusOK, module.snapshot(request.Context()))
	default:
		http.NotFound(response, request)
	}
}

func (module *Module) validate(response http.ResponseWriter, ctx context.Context, request settingsprotocol.DiagnosticsValidateRequest) {
	issues := make([]settingsprotocol.SettingsValidationIssue, 0)
	if request.ContractVersion != settingsprotocol.DiagnosticsContractVersion {
		issues = append(issues, settingsprotocol.SettingsValidationIssue{
			Field:   "contractVersion",
			Code:    "unsupported_contract_version",
			Message: "Diagnostics contract version is unsupported.",
		})
	}
	state := module.snapshot(ctx)
	if request.BaseRevision != state.Revision {
		writeError(response, http.StatusConflict, staleRevisionError())
		return
	}
	writeJSON(response, http.StatusOK, settingsprotocol.SettingsValidationResponse{
		Valid:  len(issues) == 0,
		Issues: issues,
	})
}

func (module *Module) apply(response http.ResponseWriter, ctx context.Context, request settingsprotocol.DiagnosticsApplyRequest) {
	if request.ContractVersion != settingsprotocol.DiagnosticsContractVersion {
		contractVersionError(response)
		return
	}
	state := module.snapshot(ctx)
	if state.Availability.State != settingsprotocol.SettingsAvailabilityAvailable {
		writeError(response, http.StatusServiceUnavailable, settingsprotocol.SettingsError{
			Code:      settingsprotocol.SettingsErrorUnavailable,
			Message:   state.Availability.Reason,
			Retryable: true,
		})
		return
	}
	if request.BaseRevision != state.Revision {
		writeError(response, http.StatusConflict, staleRevisionError())
		return
	}

	args := []string{"--user", "disable", "--now", overlayUnit}
	if request.Draft.DiagnosticOverlayEnabled {
		args = []string{"--user", "enable", "--now", overlayUnit}
	}
	if output, err := exec.CommandContext(ctx, module.systemctl, args...).CombinedOutput(); err != nil {
		detail := strings.TrimSpace(string(output))
		if detail == "" {
			detail = err.Error()
		}
		writeError(response, http.StatusServiceUnavailable, settingsprotocol.SettingsError{
			Code:      settingsprotocol.SettingsErrorApplyFailed,
			Message:   detail,
			Retryable: true,
		})
		return
	}

	active := module.snapshot(ctx)
	if active.Availability.State != settingsprotocol.SettingsAvailabilityAvailable {
		writeError(response, http.StatusServiceUnavailable, settingsprotocol.SettingsError{
			Code:      settingsprotocol.SettingsErrorApplyFailed,
			Message:   "overlay state could not be confirmed after apply",
			Retryable: true,
		})
		return
	}
	writeJSON(response, http.StatusOK, settingsprotocol.DiagnosticsApplyResponse{
		State: active,
		Outcome: settingsprotocol.SettingsApplyOutcome{
			Kind: settingsprotocol.SettingsApplyApplied,
		},
	})
}

func (module *Module) defaults(response http.ResponseWriter, ctx context.Context, request settingsprotocol.SettingsDefaultsRequest) {
	if request.ContractVersion != settingsprotocol.DiagnosticsContractVersion {
		contractVersionError(response)
		return
	}
	state := module.snapshot(ctx)
	if request.BaseRevision != state.Revision {
		writeError(response, http.StatusConflict, staleRevisionError())
		return
	}
	writeJSON(response, http.StatusOK, state.Defaults)
}

func (module *Module) snapshot(ctx context.Context) settingsprotocol.DiagnosticsSettingsState {
	enabledState, enabledOK := module.systemctlState(ctx, "--user", "is-enabled", overlayUnit)
	activeState, activeOK := module.systemctlState(ctx, "--user", "is-active", overlayUnit)
	availability := settingsprotocol.SettingsModuleAvailability{State: settingsprotocol.SettingsAvailabilityAvailable}
	if !enabledOK || !activeOK {
		availability = settingsprotocol.SettingsModuleAvailability{
			State:  settingsprotocol.SettingsAvailabilityUnavailable,
			Reason: "diagnostic overlay user service is unavailable",
		}
	}

	fingerprint := enabledState + "\x00" + activeState
	module.mu.Lock()
	if !module.initialized {
		module.revision = 1
		module.initialized = true
		module.fingerprint = fingerprint
	} else if fingerprint != module.fingerprint {
		module.revision++
		module.fingerprint = fingerprint
	}
	revision := module.revision
	module.mu.Unlock()

	enabled := enabledState == "enabled"
	active := activeState == "active"
	components := module.componentHealth(ctx)
	bundle := settingsprotocol.DiagnosticsSupportBundle{SchemaVersion: 1, GeneratedAtUnixMillis: uint64(time.Now().UnixMilli()), ProductVersion: productVersion, SettingsSchemaVersion: settingsprotocol.SettingsSchemaVersion, Components: append([]settingsprotocol.DiagnosticsComponentHealth(nil), components...)}
	return settingsprotocol.DiagnosticsSettingsState{
		ModuleID:        settingsprotocol.DiagnosticsModuleID,
		ContractVersion: settingsprotocol.DiagnosticsContractVersion,
		Revision:        revision,
		Active: settingsprotocol.DiagnosticsSettings{
			DiagnosticOverlayEnabled: enabled,
		},
		Defaults: settingsprotocol.DiagnosticsSettings{
			DiagnosticOverlayEnabled: false,
		},
		Service: settingsprotocol.DiagnosticsServiceState{
			Enabled:      enabled,
			Active:       active,
			EnabledState: enabledState,
			ActiveState:  activeState,
		},
		ProductVersion:        productVersion,
		SettingsSchemaVersion: settingsprotocol.SettingsSchemaVersion,
		Components:            components,
		SupportBundle:         bundle,
		Availability:          availability,
	}
}

func (module *Module) componentHealth(ctx context.Context) []settingsprotocol.DiagnosticsComponentHealth {
	userState, _ := module.systemctlState(ctx, "--user", "is-active", "agora-de-shellui.service")
	bridgeState, _ := module.systemctlState(ctx, "is-active", "compositor-bridge.service")
	displayState := "available"
	displayPath := module.displayAuthority
	if displayPath == "" {
		displayPath = "agora-de-display-authority"
	}
	if _, err := exec.LookPath(displayPath); err != nil {
		displayState = "unavailable"
	}
	state := func(value string) string {
		if value == "active" || value == "available" {
			return "available"
		}
		return "unavailable"
	}
	return []settingsprotocol.DiagnosticsComponentHealth{
		{ID: "shell-gateway", Label: "Shell gateway", State: state(userState), Version: productVersion, Detail: userState, Recovery: "Restart the Agora DE shell user service."},
		{ID: "compositor-bridge", Label: "Compositor bridge", State: state(bridgeState), Version: "layout-protocol-v1", Detail: bridgeState, Recovery: "Restart the installed compositor bridge service."},
		{ID: "display-authority", Label: "Display authority", State: state(displayState), Version: "display-contract-v1", Detail: displayState, Recovery: "Reinstall the Agora DE display authority."},
		{ID: "settings-contract", Label: "Settings contract", State: "available", Version: "1", Detail: "schema 1", Recovery: "Update shell components together if versions differ."},
	}
}

func (module *Module) systemctlState(ctx context.Context, args ...string) (string, bool) {
	output, err := exec.CommandContext(ctx, module.systemctl, args...).CombinedOutput()
	state := strings.TrimSpace(string(output))
	if state == "" && err != nil {
		return "unavailable", false
	}
	if state == "" {
		return "unknown", true
	}
	return state, true
}

func decodeStrict(request *http.Request, target any) error {
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("request must contain one JSON value")
		}
		return err
	}
	return nil
}

func writeJSON(response http.ResponseWriter, status int, payload any) {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(payload)
}

func writeError(response http.ResponseWriter, status int, settingsError settingsprotocol.SettingsError) {
	writeJSON(response, status, settingsError)
}

func invalidRequest(response http.ResponseWriter, err error) {
	writeError(response, http.StatusBadRequest, settingsprotocol.SettingsError{
		Code:      settingsprotocol.SettingsErrorInvalidRequest,
		Message:   err.Error(),
		Retryable: false,
	})
}

func contractVersionError(response http.ResponseWriter) {
	writeError(response, http.StatusUnprocessableEntity, settingsprotocol.SettingsError{
		Code:      settingsprotocol.SettingsErrorUnsupported,
		Message:   "Diagnostics contract version is unsupported.",
		Retryable: false,
	})
}

func staleRevisionError() settingsprotocol.SettingsError {
	return settingsprotocol.SettingsError{
		Code:      settingsprotocol.SettingsErrorStaleRevision,
		Message:   "Diagnostics settings changed after this draft was loaded.",
		Retryable: true,
	}
}

func methodNotAllowed(response http.ResponseWriter, method string) {
	response.Header().Set("Allow", method)
	writeError(response, http.StatusMethodNotAllowed, settingsprotocol.SettingsError{
		Code:      settingsprotocol.SettingsErrorInvalidRequest,
		Message:   "method not allowed",
		Retryable: false,
	})
}
