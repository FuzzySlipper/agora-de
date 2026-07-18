package settingswindowmanagement

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strconv"
	"strings"

	"agora-de.local/go/internal/settingsprotocol"
)

type Config struct{ CompositorctlPath string }

type Module struct{ compositorctl string }

func New(config Config) *Module {
	path := strings.TrimSpace(config.CompositorctlPath)
	if path == "" {
		path = "agora-de-compositorctl"
	}
	return &Module{compositorctl: path}
}

func (module *Module) Manifest() settingsprotocol.SettingsModuleManifest {
	return settingsprotocol.SettingsModuleManifest{
		ID:          settingsprotocol.WindowManagementModuleID,
		Category:    settingsprotocol.SettingsCategoryPersonal,
		Title:       "Window Management",
		Summary:     "Configure layouts, master sizing, and workspace gaps.",
		Icon:        "window-management",
		Route:       settingsprotocol.WindowManagementModuleID,
		SearchTerms: []string{"windows", "layout", "tiling", "master", "gaps", "workspace"},
		Capabilities: []settingsprotocol.SettingsCapability{
			settingsprotocol.SettingsCapabilityLoad,
			settingsprotocol.SettingsCapabilityValidate,
			settingsprotocol.SettingsCapabilityApply,
			settingsprotocol.SettingsCapabilityRestoreDefaults,
		},
		BackendAdapter: "settings-window-management", UIEntryPoint: "settings-window-management",
		ContractVersion: settingsprotocol.WindowManagementContractVersion,
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
			unavailable(response, err)
			return
		}
		writeJSON(response, http.StatusOK, state)
	case "/validate":
		if request.Method != http.MethodPost {
			methodNotAllowed(response, http.MethodPost)
			return
		}
		var body settingsprotocol.WindowManagementValidateRequest
		if err := decodeStrict(request, &body); err != nil {
			invalid(response, err)
			return
		}
		module.validate(response, request.Context(), body)
	case "/apply":
		if request.Method != http.MethodPost {
			methodNotAllowed(response, http.MethodPost)
			return
		}
		var body settingsprotocol.WindowManagementApplyRequest
		if err := decodeStrict(request, &body); err != nil {
			invalid(response, err)
			return
		}
		module.apply(response, request.Context(), body)
	case "/defaults":
		if request.Method != http.MethodPost {
			methodNotAllowed(response, http.MethodPost)
			return
		}
		var body settingsprotocol.SettingsDefaultsRequest
		if err := decodeStrict(request, &body); err != nil {
			invalid(response, err)
			return
		}
		state, err := module.snapshot(request.Context())
		if err != nil {
			unavailable(response, err)
			return
		}
		if body.ContractVersion != settingsprotocol.WindowManagementContractVersion {
			invalidMessage(response, "unsupported Window Management contract version")
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

func (module *Module) validate(response http.ResponseWriter, ctx context.Context, request settingsprotocol.WindowManagementValidateRequest) {
	state, err := module.snapshot(ctx)
	if err != nil {
		unavailable(response, err)
		return
	}
	if request.BaseRevision != state.Revision {
		stale(response)
		return
	}
	issues := validateSettings(request.ContractVersion, request.Draft)
	writeJSON(response, http.StatusOK, settingsprotocol.SettingsValidationResponse{Valid: len(issues) == 0, Issues: issues})
}

func (module *Module) apply(response http.ResponseWriter, ctx context.Context, request settingsprotocol.WindowManagementApplyRequest) {
	state, err := module.snapshot(ctx)
	if err != nil {
		unavailable(response, err)
		return
	}
	if request.BaseRevision != state.Revision {
		stale(response)
		return
	}
	if issues := validateSettings(request.ContractVersion, request.Draft); len(issues) != 0 {
		writeJSON(response, http.StatusBadRequest, settingsprotocol.SettingsError{Code: settingsprotocol.SettingsErrorValidation, Message: "Window Management settings are invalid", Retryable: false, Issues: issues})
		return
	}
	draft := request.Draft
	args := []string{"layout", "set-settings", "--rule", string(draft.Rule), "--mode", string(draft.Mode),
		"--outer-horizontal", strconv.Itoa(int(draft.Gaps.OuterHorizontal)), "--outer-vertical", strconv.Itoa(int(draft.Gaps.OuterVertical)),
		"--inner-horizontal", strconv.Itoa(int(draft.Gaps.InnerHorizontal)), "--inner-vertical", strconv.Itoa(int(draft.Gaps.InnerVertical)),
		"--master-count", strconv.Itoa(int(draft.MasterCount)), "--master-ratio", strconv.FormatFloat(draft.MasterRatio, 'f', 2, 64),
		fmt.Sprintf("--smart-gaps=%t", draft.SmartGaps), "--timeout-ms", "2000"}
	if output, err := exec.CommandContext(ctx, module.compositorctl, args...).CombinedOutput(); err != nil {
		detail := strings.TrimSpace(string(output))
		if detail == "" {
			detail = err.Error()
		}
		writeJSON(response, http.StatusServiceUnavailable, settingsprotocol.SettingsError{Code: settingsprotocol.SettingsErrorApplyFailed, Message: detail, Retryable: true})
		return
	}
	active, err := module.snapshot(ctx)
	if err != nil {
		unavailable(response, fmt.Errorf("confirm applied layout: %w", err))
		return
	}
	writeJSON(response, http.StatusOK, settingsprotocol.WindowManagementApplyResponse{State: active, Outcome: settingsprotocol.SettingsApplyOutcome{Kind: settingsprotocol.SettingsApplyApplied}})
}

func validateSettings(contract uint16, draft settingsprotocol.WindowManagementSettings) []settingsprotocol.SettingsValidationIssue {
	issues := make([]settingsprotocol.SettingsValidationIssue, 0)
	if contract != settingsprotocol.WindowManagementContractVersion {
		issues = append(issues, issue("contractVersion", "unsupported_contract_version", "Window Management contract version is unsupported."))
	}
	if draft.Mode != settingsprotocol.WindowLayoutModeFreeform && draft.Mode != settingsprotocol.WindowLayoutModeZones && draft.Mode != settingsprotocol.WindowLayoutModeColumns {
		issues = append(issues, issue("mode", "unsupported", "Choose a supported layout mode."))
	}
	if draft.Rule != settingsprotocol.WindowLayoutRuleZones && draft.Rule != settingsprotocol.WindowLayoutRuleMasterStack && draft.Rule != settingsprotocol.WindowLayoutRuleDwindle {
		issues = append(issues, issue("rule", "unsupported", "Choose a supported layout rule."))
	}
	if draft.MasterCount < 1 || draft.MasterCount > 8 {
		issues = append(issues, issue("masterCount", "out_of_range", "Master count must be between 1 and 8."))
	}
	if draft.MasterRatio < .1 || draft.MasterRatio > .9 {
		issues = append(issues, issue("masterRatio", "out_of_range", "Master ratio must be between 10% and 90%."))
	}
	for field, value := range map[string]uint16{"gaps.outerHorizontal": draft.Gaps.OuterHorizontal, "gaps.outerVertical": draft.Gaps.OuterVertical, "gaps.innerHorizontal": draft.Gaps.InnerHorizontal, "gaps.innerVertical": draft.Gaps.InnerVertical} {
		if value > 128 {
			issues = append(issues, issue(field, "out_of_range", "Gaps must be between 0 and 128 pixels."))
		}
	}
	return issues
}

type compositorState struct {
	Layout struct {
		Mode     string `json:"mode"`
		Revision uint64 `json:"revision"`
		Settings struct {
			Rule string `json:"rule"`
			Mode string `json:"mode"`
			Gaps struct {
				OuterHorizontal uint16 `json:"outer_horizontal"`
				OuterVertical   uint16 `json:"outer_vertical"`
				InnerHorizontal uint16 `json:"inner_horizontal"`
				InnerVertical   uint16 `json:"inner_vertical"`
			} `json:"gaps"`
			MasterCount uint8   `json:"master_count"`
			MasterRatio float64 `json:"master_ratio"`
			SmartGaps   bool    `json:"smart_gaps"`
		} `json:"settings"`
		Workspaces []struct {
			ID           string   `json:"id"`
			Name         string   `json:"name"`
			OutputID     string   `json:"output_id"`
			Active       bool     `json:"active"`
			SurfaceOrder []string `json:"surface_order"`
		} `json:"workspaces"`
	} `json:"layout"`
}

func (module *Module) snapshot(ctx context.Context) (settingsprotocol.WindowManagementSettingsState, error) {
	output, err := exec.CommandContext(ctx, module.compositorctl, "layout", "get").CombinedOutput()
	if err != nil {
		return settingsprotocol.WindowManagementSettingsState{}, fmt.Errorf("layout authority unavailable: %s", strings.TrimSpace(string(output)))
	}
	var source compositorState
	if err := json.Unmarshal(output, &source); err != nil {
		return settingsprotocol.WindowManagementSettingsState{}, fmt.Errorf("decode layout authority: %w", err)
	}
	settings := settingsprotocol.WindowManagementSettings{Mode: settingsprotocol.WindowLayoutMode(source.Layout.Settings.Mode), Rule: settingsprotocol.WindowLayoutRule(source.Layout.Settings.Rule), MasterCount: source.Layout.Settings.MasterCount, MasterRatio: source.Layout.Settings.MasterRatio, SmartGaps: source.Layout.Settings.SmartGaps, Gaps: settingsprotocol.WindowManagementGaps{OuterHorizontal: source.Layout.Settings.Gaps.OuterHorizontal, OuterVertical: source.Layout.Settings.Gaps.OuterVertical, InnerHorizontal: source.Layout.Settings.Gaps.InnerHorizontal, InnerVertical: source.Layout.Settings.Gaps.InnerVertical}}
	workspaces := make([]settingsprotocol.WindowWorkspaceSummary, 0, len(source.Layout.Workspaces))
	for _, workspace := range source.Layout.Workspaces {
		workspaces = append(workspaces, settingsprotocol.WindowWorkspaceSummary{ID: workspace.ID, Name: workspace.Name, OutputID: workspace.OutputID, Active: workspace.Active, SurfaceCount: uint32(len(workspace.SurfaceOrder))})
	}
	return settingsprotocol.WindowManagementSettingsState{ModuleID: settingsprotocol.WindowManagementModuleID, ContractVersion: settingsprotocol.WindowManagementContractVersion, Revision: source.Layout.Revision, Active: settings, Defaults: defaults(), Workspaces: workspaces, Availability: settingsprotocol.SettingsModuleAvailability{State: settingsprotocol.SettingsAvailabilityAvailable}}, nil
}

func defaults() settingsprotocol.WindowManagementSettings {
	return settingsprotocol.WindowManagementSettings{Mode: settingsprotocol.WindowLayoutModeZones, Rule: settingsprotocol.WindowLayoutRuleMasterStack, Gaps: settingsprotocol.WindowManagementGaps{}, MasterCount: 1, MasterRatio: .5, SmartGaps: true}
}
func issue(field, code, message string) settingsprotocol.SettingsValidationIssue {
	return settingsprotocol.SettingsValidationIssue{Field: field, Code: code, Message: message}
}
func stale(response http.ResponseWriter) {
	writeJSON(response, http.StatusConflict, settingsprotocol.SettingsError{Code: settingsprotocol.SettingsErrorStaleRevision, Message: "layout state changed; reload before applying", Retryable: true})
}
func unavailable(response http.ResponseWriter, err error) {
	writeJSON(response, http.StatusServiceUnavailable, settingsprotocol.SettingsError{Code: settingsprotocol.SettingsErrorUnavailable, Message: err.Error(), Retryable: true})
}
func invalid(response http.ResponseWriter, err error) { invalidMessage(response, err.Error()) }
func invalidMessage(response http.ResponseWriter, message string) {
	writeJSON(response, http.StatusBadRequest, settingsprotocol.SettingsError{Code: settingsprotocol.SettingsErrorInvalidRequest, Message: message, Retryable: false})
}
func methodNotAllowed(response http.ResponseWriter, method string) {
	response.Header().Set("Allow", method)
	invalidMessage(response, "method not allowed")
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
