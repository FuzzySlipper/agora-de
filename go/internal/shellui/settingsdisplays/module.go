package settingsdisplays

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"path/filepath"
	"strings"

	"agora-de.local/go/internal/settingsprotocol"
)

type Config struct {
	AuthorityPath string
	StateDir      string
}

type Module struct {
	authorityPath string
	stateDir      string
}

func New(config Config) *Module {
	path := strings.TrimSpace(config.AuthorityPath)
	if path == "" {
		path = "agora-de-display-authority"
	}
	stateDir := strings.TrimSpace(config.StateDir)
	if stateDir == "" {
		stateDir = filepath.Join(".", "display-state")
	}
	return &Module{authorityPath: path, stateDir: filepath.Join(stateDir, "displays")}
}

func (module *Module) Manifest() settingsprotocol.SettingsModuleManifest {
	return settingsprotocol.SettingsModuleManifest{
		ID:          settingsprotocol.DisplaysModuleID,
		Category:    settingsprotocol.SettingsCategoryHardware,
		Title:       "Displays",
		Summary:     "Arrange displays and configure modes, scale, and rotation safely.",
		Icon:        "display",
		Route:       settingsprotocol.DisplaysModuleID,
		SearchTerms: []string{"monitor", "resolution", "refresh", "scale", "rotation", "arrangement"},
		Capabilities: []settingsprotocol.SettingsCapability{
			settingsprotocol.SettingsCapabilityLoad,
			settingsprotocol.SettingsCapabilityValidate,
			settingsprotocol.SettingsCapabilityApply,
			settingsprotocol.SettingsCapabilityRestoreDefaults,
		},
		BackendAdapter:  "settings-displays",
		UIEntryPoint:    "settings-displays",
		ContractVersion: settingsprotocol.DisplaysContractVersion,
	}
}

func (module *Module) Availability(ctx context.Context) settingsprotocol.SettingsModuleAvailability {
	state, err := module.snapshot(ctx)
	if err != nil {
		return settingsprotocol.SettingsModuleAvailability{State: settingsprotocol.SettingsAvailabilityUnavailable, Reason: err.Error()}
	}
	return state.Availability
}

func (module *Module) Handler() http.Handler { return http.HandlerFunc(module.serveHTTP) }

func (module *Module) serveHTTP(response http.ResponseWriter, request *http.Request) {
	switch request.URL.Path {
	case "/state":
		if request.Method != http.MethodGet {
			methodNotAllowed(response, http.MethodGet)
			return
		}
		state, err := module.snapshot(request.Context())
		if err != nil {
			module.writeAuthorityError(response, err)
			return
		}
		writeJSON(response, http.StatusOK, state)
	case "/validate":
		module.forwardMutation(response, request, "validate", &settingsprotocol.DisplayValidateRequest{}, &settingsprotocol.SettingsValidationResponse{})
	case "/apply":
		module.forwardMutation(response, request, "apply", &settingsprotocol.DisplayApplyRequest{}, &settingsprotocol.DisplayApplyResponse{})
	case "/keep", "/revert":
		module.forwardMutation(response, request, strings.TrimPrefix(request.URL.Path, "/"), &settingsprotocol.DisplayLeaseActionRequest{}, &settingsprotocol.DisplayApplyResponse{})
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
		state, err := module.snapshot(request.Context())
		if err != nil {
			module.writeAuthorityError(response, err)
			return
		}
		if defaults.ContractVersion != settingsprotocol.DisplaysContractVersion {
			writeJSON(response, http.StatusBadRequest, settingsprotocol.SettingsError{Code: settingsprotocol.SettingsErrorInvalidRequest, Message: "unsupported displays contract version", Retryable: false})
			return
		}
		if defaults.BaseRevision != state.Revision {
			writeJSON(response, http.StatusConflict, settingsprotocol.SettingsError{Code: settingsprotocol.SettingsErrorStaleRevision, Message: "display state changed; reload before applying", Retryable: true})
			return
		}
		writeJSON(response, http.StatusOK, state.Defaults)
	default:
		http.NotFound(response, request)
	}
}

func (module *Module) forwardMutation(response http.ResponseWriter, request *http.Request, command string, input any, output any) {
	if request.Method != http.MethodPost {
		methodNotAllowed(response, http.MethodPost)
		return
	}
	if err := decodeStrict(request, input); err != nil {
		invalidRequest(response, err)
		return
	}
	payload, err := json.Marshal(input)
	if err != nil {
		invalidRequest(response, err)
		return
	}
	if err := module.command(request.Context(), command, payload, output); err != nil {
		module.writeAuthorityError(response, err)
		return
	}
	writeJSON(response, http.StatusOK, output)
}

func (module *Module) snapshot(ctx context.Context) (settingsprotocol.DisplaySettingsState, error) {
	var state settingsprotocol.DisplaySettingsState
	if err := module.command(ctx, "snapshot", nil, &state); err != nil {
		return settingsprotocol.DisplaySettingsState{}, err
	}
	if state.ModuleID != settingsprotocol.DisplaysModuleID || state.ContractVersion != settingsprotocol.DisplaysContractVersion {
		return settingsprotocol.DisplaySettingsState{}, fmt.Errorf("display authority returned an incompatible contract")
	}
	if state.Availability.State == settingsprotocol.SettingsAvailabilityAvailable && !state.Capabilities.OutputManagement {
		return settingsprotocol.DisplaySettingsState{}, fmt.Errorf("display authority claimed available without output-management")
	}
	return state, nil
}

type authorityError struct {
	contract settingsprotocol.SettingsError
	cause    error
}

func (err authorityError) Error() string {
	if err.contract.Message != "" {
		return err.contract.Message
	}
	return err.cause.Error()
}

func (module *Module) command(ctx context.Context, operation string, input []byte, target any) error {
	command := exec.CommandContext(ctx, module.authorityPath, operation, "--state-dir", module.stateDir)
	if input != nil {
		command.Stdin = bytes.NewReader(input)
	}
	payload, err := command.CombinedOutput()
	if err != nil {
		var contract settingsprotocol.SettingsError
		if decodeJSON(payload, &contract) == nil && contract.Code != "" {
			return authorityError{contract: contract, cause: err}
		}
		detail := strings.TrimSpace(string(payload))
		if detail == "" {
			detail = err.Error()
		}
		return fmt.Errorf("display authority unavailable: %s", detail)
	}
	if err := decodeJSON(payload, target); err != nil {
		return fmt.Errorf("decode display authority response: %w", err)
	}
	return nil
}

func (module *Module) writeAuthorityError(response http.ResponseWriter, err error) {
	var typed authorityError
	if errors.As(err, &typed) {
		status := http.StatusServiceUnavailable
		switch typed.contract.Code {
		case settingsprotocol.SettingsErrorInvalidRequest, settingsprotocol.SettingsErrorValidation:
			status = http.StatusBadRequest
		case settingsprotocol.SettingsErrorStaleRevision, settingsprotocol.SettingsErrorTransactionBusy, settingsprotocol.SettingsErrorConfirmationExpired:
			status = http.StatusConflict
		case settingsprotocol.SettingsErrorTimeout:
			status = http.StatusGatewayTimeout
		}
		writeJSON(response, status, typed.contract)
		return
	}
	writeJSON(response, http.StatusServiceUnavailable, settingsprotocol.SettingsError{Code: settingsprotocol.SettingsErrorUnavailable, Message: err.Error(), Retryable: true})
}

func decodeStrict(request *http.Request, target any) error {
	decoder := json.NewDecoder(io.LimitReader(request.Body, 1<<20))
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

func decodeJSON(payload []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("authority response must contain one JSON value")
	}
	return nil
}

func methodNotAllowed(response http.ResponseWriter, method string) {
	response.Header().Set("Allow", method)
	writeJSON(response, http.StatusMethodNotAllowed, settingsprotocol.SettingsError{Code: settingsprotocol.SettingsErrorInvalidRequest, Message: "method not allowed", Retryable: false})
}

func invalidRequest(response http.ResponseWriter, err error) {
	writeJSON(response, http.StatusBadRequest, settingsprotocol.SettingsError{Code: settingsprotocol.SettingsErrorInvalidRequest, Message: err.Error(), Retryable: false})
}

func writeJSON(response http.ResponseWriter, status int, payload any) {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(payload)
}
