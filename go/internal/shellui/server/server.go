package server

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"agora-de.local/go/internal/appcatalog"
	"agora-de.local/go/internal/nativelaunch"
	"agora-de.local/go/internal/session"
	"agora-de.local/go/internal/shellui/catalog"
	"agora-de.local/go/internal/shellui/catalogroute"
	"agora-de.local/go/internal/shellui/staticserve"
	"agora-de.local/go/internal/shellui/surfaceroute"
	"agora-de.local/go/internal/shellui/surfaces"
	"agora-de.local/go/internal/shellui/theme"
)

const (
	DefaultListenAddress = "127.0.0.1:7780"
	WorkControlsPath     = "/api/work-surface-controls"
	SurfaceActionPath    = "/api/surfaces/action"
	OperatorStatusPath   = "/api/operator/status"
	WorkspacesPath       = "/api/workspaces"
	WorkspaceActionPath  = "/api/workspaces/action"

	SurfaceProviderFixture                      = "fixture"
	SurfaceProviderCompositorctl                = "compositorctl"
	CatalogProviderFixture                      = "fixture"
	CatalogProviderDesktopEntries               = "desktop_entries"
	NativeLaunchProviderDisabled                = "disabled"
	NativeLaunchProviderStructuredCompositorctl = "structured_compositorctl"
)

type Config struct {
	StaticRoot               string
	FixtureProviders         bool
	CatalogProvider          string
	DesktopEntryRoots        []string
	SurfaceProvider          string
	CompositorctlPath        string
	NativeLaunchProvider     string
	NativeLaunchAllowlist    []string
	NativeLaunchRequesterUID int
	NativeLaunchRequesterGID int
	NativeLaunchSessionToken string
	NativeLaunchOutputName   string
	NativeLaunchHome         string
	NativeLaunchWorkingDir   string
}

func NewHandler(config Config) (http.Handler, error) {
	catalogProvider, launchProvider, surfaceProvider, err := providers(config)
	if err != nil {
		return nil, err
	}

	mux := http.NewServeMux()
	mux.Handle(catalogroute.AppsPath, catalogroute.New(catalogProvider, launchProvider))
	mux.Handle(catalogroute.LaunchPath, catalogroute.New(catalogProvider, launchProvider))
	mux.Handle(surfaceroute.SurfacesPath, surfaceroute.New(surfaceProvider))
	mux.Handle(WorkControlsPath, surfaceroute.Handler{
		Path:     WorkControlsPath,
		Provider: surfaceProvider,
	})
	mux.Handle(SurfaceActionPath, surfaceActionHandler(config))
	mux.Handle(OperatorStatusPath, operatorStatusHandler(config, surfaceProvider))
	mux.Handle(WorkspacesPath, workspacesHandler(surfaceProvider))
	mux.Handle(WorkspaceActionPath, workspaceActionHandler(surfaceProvider))
	mux.Handle("/shell/dist/", shellAssetHandler(config.StaticRoot))
	return noStore(mux), nil
}

func providers(config Config) (catalogroute.Provider, catalogroute.LaunchProvider, surfaceroute.Provider, error) {
	if !config.FixtureProviders {
		return nil, nil, nil, fmt.Errorf("shellui live providers are not wired yet; enable fixture providers for deployment testing")
	}

	appCatalog, err := catalogSource(config)
	if err != nil {
		return nil, nil, nil, err
	}
	apps, err := launchAwareAppViews(config, appCatalog)
	if err != nil {
		return nil, nil, nil, err
	}
	surfaceProvider, err := surfaceProvider(config)
	if err != nil {
		return nil, nil, nil, err
	}

	catalogProvider := func(*http.Request) ([]catalog.AppView, error) {
		return apps, nil
	}
	return catalogProvider, launchProvider(config, appCatalog), surfaceProvider, nil
}

func catalogSource(config Config) (*appcatalog.Catalog, error) {
	mode := strings.TrimSpace(config.CatalogProvider)
	if mode == "" {
		mode = CatalogProviderFixture
	}
	switch mode {
	case CatalogProviderFixture:
		return fixtureCatalog(), nil
	case CatalogProviderDesktopEntries:
		return appcatalog.ImportDesktopEntries(config.DesktopEntryRoots...)
	default:
		return nil, fmt.Errorf("unknown catalog provider %q", mode)
	}
}

func nativeLaunchMode(config Config) (string, error) {
	mode := strings.TrimSpace(config.NativeLaunchProvider)
	if mode == "" {
		mode = NativeLaunchProviderDisabled
	}
	switch mode {
	case NativeLaunchProviderDisabled, NativeLaunchProviderStructuredCompositorctl:
		return mode, nil
	default:
		return "", fmt.Errorf("unknown native launch provider %q", mode)
	}
}

func setFrom(values []string) map[string]bool {
	set := make(map[string]bool, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			set[value] = true
		}
	}
	return set
}

func environmentMap(values []string) map[string]string {
	environment := make(map[string]string, len(values))
	for _, value := range values {
		key, raw, ok := strings.Cut(value, "=")
		if ok && key != "" {
			environment[key] = raw
		}
	}
	return environment
}

func launchAwareAppViews(config Config, source *appcatalog.Catalog) ([]catalog.AppView, error) {
	views := catalog.VisibleAppViews(source)
	targets := launchTargets()
	nativeMode, err := nativeLaunchMode(config)
	if err != nil {
		return nil, err
	}
	nativeAllowlist := setFrom(config.NativeLaunchAllowlist)
	for index := range views {
		if _, ok := targets[views[index].ID]; ok {
			continue
		}
		entry, ok := source.Get(views[index].ID)
		launchable := ok &&
			nativeMode == NativeLaunchProviderStructuredCompositorctl &&
			nativeAllowlist[views[index].ID] &&
			nativelaunch.CanPrepare(entry)
		views[index].Launchable = launchable
		if !launchable {
			views[index].DisabledReason = nativeDisabledReason(nativeMode, nativeAllowlist[views[index].ID], entry)
		}
	}
	return views, nil
}

func nativeDisabledReason(mode string, allowlisted bool, entry appcatalog.Entry) string {
	switch {
	case !nativelaunch.CanPrepare(entry):
		return "unsupported desktop entry"
	case mode == NativeLaunchProviderDisabled:
		return "native launch disabled"
	case !allowlisted:
		return "not enabled for native launch"
	default:
		return "not launchable"
	}
}

func fixtureCatalog() *appcatalog.Catalog {
	source := appcatalog.NewCatalog()
	source.Add(appcatalog.Entry{
		ID:         "example-browser",
		Name:       "Example Browser",
		Exec:       "example-browser --new-window %u",
		Icon:       "example-browser",
		Categories: []string{"Network", "WebBrowser"},
	})
	source.Add(appcatalog.Entry{
		ID:         "shell-status",
		Name:       "Shell Status",
		Exec:       "agora-de-shell-status",
		Icon:       "preferences-system",
		Categories: []string{"System", "Settings"},
	})
	return source
}

type launchTarget struct {
	URL   string
	Title string
	AppID string
}

func launchTargets() map[string]launchTarget {
	return map[string]launchTarget{
		"example-browser": {
			URL:   "http://127.0.0.1:17780/shell/dist/desktop/?surface=app-example",
			Title: "Agora DE Example Browser",
			AppID: "io.agorade.ExampleBrowser",
		},
		"shell-status": {
			URL:   "http://127.0.0.1:17780/shell/dist/desktop/?surface=operator",
			Title: "Agora DE Shell Status",
			AppID: "io.agorade.ShellStatus",
		},
	}
}

func launchProvider(config Config, appCatalog *appcatalog.Catalog) catalogroute.LaunchProvider {
	path := strings.TrimSpace(config.CompositorctlPath)
	if path == "" {
		path = "compositorctl"
	}
	targets := launchTargets()
	nativeAllowlist := setFrom(config.NativeLaunchAllowlist)
	return func(request *http.Request, launch catalogroute.LaunchRequest) (catalogroute.LaunchResult, error) {
		target, ok := targets[launch.AppID]
		if ok {
			return launchWebviewTarget(request, path, launch.AppID, target)
		}
		entry, ok := appCatalog.Get(launch.AppID)
		if !ok || entry.NoDisplay {
			return catalogroute.LaunchResult{}, fmt.Errorf("app %q not found", launch.AppID)
		}

		nativeMode, err := nativeLaunchMode(config)
		if err != nil {
			return catalogroute.LaunchResult{}, err
		}
		if nativeMode != NativeLaunchProviderStructuredCompositorctl || !nativeAllowlist[launch.AppID] {
			return catalogroute.LaunchResult{}, fmt.Errorf("app %q is not launchable by shellui", launch.AppID)
		}
		return launchNativeTarget(request, config, path, launch.AppID, entry)
	}
}

func launchWebviewTarget(request *http.Request, path string, appID string, target launchTarget) (catalogroute.LaunchResult, error) {
	ctx, cancel := context.WithTimeout(request.Context(), 8*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, path,
		"launch",
		"--url", target.URL,
		"--webview-title", target.Title,
		"--app-id", target.AppID,
		"--expected-app-id", target.AppID,
		"--wait-surface",
		"--wait-timeout-ms", "5000",
	).Output()
	if err != nil {
		return catalogroute.LaunchResult{}, fmt.Errorf("compositorctl launch: %w", err)
	}
	result, err := decodeCompositorctlLaunch(output)
	if err != nil {
		return catalogroute.LaunchResult{}, err
	}
	result.AppID = appID
	if result.Status == "" {
		result.Status = "launched"
	}
	return result, nil
}

func launchNativeTarget(request *http.Request, config Config, path string, appID string, entry appcatalog.Entry) (catalogroute.LaunchResult, error) {
	ctx, cancel := context.WithTimeout(request.Context(), 8*time.Second)
	defer cancel()
	result, err := nativelaunch.New(nativelaunch.CompositorctlBridge{Path: path}).Launch(ctx, nativelaunch.Request{
		Entry:              entry,
		RequesterUID:       config.NativeLaunchRequesterUID,
		RequesterGID:       config.NativeLaunchRequesterGID,
		SessionToken:       session.Token(config.NativeLaunchSessionToken),
		AuditCorrelationID: "shellui:" + appID,
		OutputName:         config.NativeLaunchOutputName,
		WorkingDirectory:   config.NativeLaunchWorkingDir,
		HomeDirectory:      config.NativeLaunchHome,
		BaseEnvironment:    environmentMap(os.Environ()),
	})
	if err != nil {
		return catalogroute.LaunchResult{}, err
	}
	return catalogroute.LaunchResult{
		AppID:     appID,
		LaunchID:  result.LaunchID,
		SurfaceID: result.SurfaceID,
		Status:    string(result.Status),
	}, nil
}

func surfaceProvider(config Config) (surfaceroute.Provider, error) {
	mode := strings.TrimSpace(config.SurfaceProvider)
	if mode == "" {
		mode = SurfaceProviderFixture
	}

	switch mode {
	case SurfaceProviderFixture:
		surfaceViews := []surfaces.SurfaceView{
			{
				ID:               "view-42",
				OwnerUID:         60001,
				Mapped:           true,
				Focused:          true,
				InputDeniedCount: 1,
			},
		}
		return func(*http.Request) ([]surfaces.SurfaceView, error) {
			return surfaceViews, nil
		}, nil
	case SurfaceProviderCompositorctl:
		path := strings.TrimSpace(config.CompositorctlPath)
		if path == "" {
			path = "compositorctl"
		}
		return func(request *http.Request) ([]surfaces.SurfaceView, error) {
			ctx, cancel := context.WithTimeout(request.Context(), 3*time.Second)
			defer cancel()
			output, err := exec.CommandContext(ctx, path, "list-surfaces").Output()
			if err != nil {
				return nil, fmt.Errorf("compositorctl list-surfaces: %w", err)
			}
			return decodeCompositorctlSurfaces(output)
		}, nil
	default:
		return nil, fmt.Errorf("unknown surface provider %q", mode)
	}
}

type compositorctlListSurfacesResponse struct {
	Surfaces []compositorctlTrackedSurface `json:"surfaces"`
}

type compositorctlTrackedSurface struct {
	Surface struct {
		ID          string `json:"id"`
		AppID       string `json:"app_id"`
		Title       string `json:"title"`
		Role        string `json:"role"`
		SurfaceKind string `json:"surface_kind"`
		Visible     bool   `json:"visible"`
	} `json:"surface"`
	Client struct {
		PID int `json:"pid"`
		UID int `json:"uid"`
	} `json:"client"`
	LaunchID           string `json:"launch_id"`
	LastEvent          string `json:"last_event"`
	Focused            bool   `json:"focused"`
	Visible            bool   `json:"visible"`
	FrameCount         int    `json:"frame_count"`
	ContentCommitCount int    `json:"content_commit_count"`
}

func decodeCompositorctlSurfaces(payload []byte) ([]surfaces.SurfaceView, error) {
	var response compositorctlListSurfacesResponse
	if err := json.Unmarshal(payload, &response); err != nil {
		return nil, fmt.Errorf("decode compositorctl surfaces: %w", err)
	}
	views := make([]surfaces.SurfaceView, 0, len(response.Surfaces))
	for _, tracked := range response.Surfaces {
		if tracked.Surface.ID == "" {
			return nil, fmt.Errorf("compositorctl surface missing id")
		}
		mapped := tracked.Visible || tracked.Surface.Visible || tracked.LastEvent != "unmapped"
		if tracked.Client.PID > 0 && !processExists(tracked.Client.PID) {
			mapped = false
		}
		views = append(views, surfaces.SurfaceView{
			ID:                 tracked.Surface.ID,
			AppID:              tracked.Surface.AppID,
			Title:              tracked.Surface.Title,
			Role:               tracked.Surface.Role,
			SurfaceKind:        tracked.Surface.SurfaceKind,
			LaunchID:           tracked.LaunchID,
			OwnerUID:           tracked.Client.UID,
			Mapped:             mapped,
			Focused:            tracked.Focused,
			InputDeniedCount:   0,
			FrameCount:         tracked.FrameCount,
			ContentCommitCount: tracked.ContentCommitCount,
		})
	}
	return views, nil
}

func processExists(pid int) bool {
	if pid <= 0 {
		return false
	}
	_, err := os.Stat(fmt.Sprintf("/proc/%d", pid))
	return err == nil
}

type compositorctlLaunchResponse struct {
	LaunchID string `json:"launch_id"`
	Surface  struct {
		Surface struct {
			ID string `json:"id"`
		} `json:"surface"`
	} `json:"surface"`
}

func decodeCompositorctlLaunch(payload []byte) (catalogroute.LaunchResult, error) {
	var response compositorctlLaunchResponse
	if err := json.Unmarshal(payload, &response); err != nil {
		return catalogroute.LaunchResult{}, fmt.Errorf("decode compositorctl launch: %w", err)
	}
	if response.LaunchID == "" {
		return catalogroute.LaunchResult{}, fmt.Errorf("compositorctl launch missing launch_id")
	}
	return catalogroute.LaunchResult{
		LaunchID:  response.LaunchID,
		SurfaceID: response.Surface.Surface.ID,
		Status:    "launched",
	}, nil
}

type surfaceActionRequest struct {
	Action    string `json:"action"`
	SurfaceID string `json:"surfaceId"`
}

type surfaceActionResponse struct {
	Action    string `json:"action"`
	SurfaceID string `json:"surfaceId"`
	Status    string `json:"status"`
}

func surfaceActionHandler(config Config) http.Handler {
	path := strings.TrimSpace(config.CompositorctlPath)
	if path == "" {
		path = "compositorctl"
	}
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != SurfaceActionPath {
			http.NotFound(response, request)
			return
		}
		if request.Method != http.MethodPost {
			response.Header().Set("Allow", http.MethodPost)
			writeJSON(response, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		var action surfaceActionRequest
		if err := json.NewDecoder(request.Body).Decode(&action); err != nil {
			writeJSON(response, http.StatusBadRequest, map[string]string{"error": "invalid surface action request"})
			return
		}
		action.SurfaceID = strings.TrimSpace(action.SurfaceID)
		if action.SurfaceID == "" {
			writeJSON(response, http.StatusBadRequest, map[string]string{"error": "surfaceId is required"})
			return
		}
		args, ok := surfaceActionArgs(action)
		if !ok {
			writeJSON(response, http.StatusBadRequest, map[string]string{"error": "unsupported surface action"})
			return
		}
		ctx, cancel := context.WithTimeout(request.Context(), 5*time.Second)
		defer cancel()
		if output, err := exec.CommandContext(ctx, path, args...).CombinedOutput(); err != nil {
			message := strings.TrimSpace(string(output))
			if message == "" {
				message = err.Error()
			}
			writeJSON(response, http.StatusServiceUnavailable, map[string]string{"error": message})
			return
		}
		writeJSON(response, http.StatusAccepted, surfaceActionResponse{
			Action:    action.Action,
			SurfaceID: action.SurfaceID,
			Status:    "accepted",
		})
	})
}

func surfaceActionArgs(action surfaceActionRequest) ([]string, bool) {
	switch action.Action {
	case "focus":
		return []string{"surface", "focus", "--surface", action.SurfaceID, "--timeout-ms", "2000"}, true
	case "close":
		return []string{"surface", "close", "--surface", action.SurfaceID, "--timeout-ms", "2000"}, true
	default:
		return nil, false
	}
}

type workspacesResponse struct {
	CurrentWorkspaceID string          `json:"currentWorkspaceId"`
	Workspaces         []workspaceView `json:"workspaces"`
}

type workspaceView struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Active       bool   `json:"active"`
	SurfaceCount int    `json:"surfaceCount"`
}

type workspaceActionRequest struct {
	Action      string `json:"action"`
	WorkspaceID string `json:"workspaceId"`
}

type workspaceActionResponse struct {
	Action             string          `json:"action"`
	WorkspaceID        string          `json:"workspaceId"`
	CurrentWorkspaceID string          `json:"currentWorkspaceId"`
	Status             string          `json:"status"`
	Workspace          workspaceView   `json:"workspace"`
	Workspaces         []workspaceView `json:"workspaces"`
}

func workspacesHandler(surfaceProvider surfaceroute.Provider) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != WorkspacesPath {
			http.NotFound(response, request)
			return
		}
		if request.Method != http.MethodGet {
			response.Header().Set("Allow", http.MethodGet)
			writeJSON(response, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		writeJSON(response, http.StatusOK, collectWorkspaceState(request, surfaceProvider))
	})
}

func workspaceActionHandler(surfaceProvider surfaceroute.Provider) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != WorkspaceActionPath {
			http.NotFound(response, request)
			return
		}
		if request.Method != http.MethodPost {
			response.Header().Set("Allow", http.MethodPost)
			writeJSON(response, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		var action workspaceActionRequest
		if err := json.NewDecoder(request.Body).Decode(&action); err != nil {
			writeJSON(response, http.StatusBadRequest, map[string]string{"error": "invalid workspace action request"})
			return
		}
		if action.Action != "activate" || action.WorkspaceID != "workspace-1" {
			writeJSON(response, http.StatusBadRequest, map[string]string{"error": "unsupported workspace action"})
			return
		}
		state := collectWorkspaceState(request, surfaceProvider)
		writeJSON(response, http.StatusAccepted, workspaceActionResponse{
			Action:             action.Action,
			WorkspaceID:        action.WorkspaceID,
			CurrentWorkspaceID: state.CurrentWorkspaceID,
			Status:             "accepted",
			Workspace:          state.Workspaces[0],
			Workspaces:         state.Workspaces,
		})
	})
}

func collectWorkspaceState(request *http.Request, surfaceProvider surfaceroute.Provider) workspacesResponse {
	surfaceCount := 0
	if surfaceProvider != nil {
		if views, err := surfaceProvider(request); err == nil {
			for _, view := range views {
				if view.Mapped && view.SurfaceKind != "layer_shell" {
					surfaceCount++
				}
			}
		}
	}
	workspace := workspaceView{
		ID:           "workspace-1",
		Name:         "workspace 1",
		Active:       true,
		SurfaceCount: surfaceCount,
	}
	return workspacesResponse{
		CurrentWorkspaceID: workspace.ID,
		Workspaces:         []workspaceView{workspace},
	}
}

type operatorStatusResponse struct {
	GeneratedAtUnixMillis int64                  `json:"generatedAtUnixMillis"`
	Overall               string                 `json:"overall"`
	Services              []operatorServiceView  `json:"services"`
	Sockets               []operatorSocketView   `json:"sockets"`
	Outputs               []operatorOutputView   `json:"outputs"`
	Surfaces              operatorSurfaceSummary `json:"surfaces"`
	Recovery              operatorRecoveryView   `json:"recovery"`
}

type operatorServiceView struct {
	Name  string `json:"name"`
	Scope string `json:"scope"`
	State string `json:"state"`
}

type operatorSocketView struct {
	Path  string `json:"path"`
	State string `json:"state"`
}

type operatorOutputView struct {
	Name         string `json:"name"`
	State        string `json:"state"`
	Mode         string `json:"mode,omitempty"`
	Width        int    `json:"width,omitempty"`
	Height       int    `json:"height,omitempty"`
	SurfaceCount int    `json:"surfaceCount,omitempty"`
	Detail       string `json:"detail,omitempty"`
}

type operatorSurfaceSummary struct {
	State      string `json:"state"`
	Total      int    `json:"total"`
	LayerShell int    `json:"layerShell"`
	Work       int    `json:"work"`
	Focused    int    `json:"focused"`
	Detail     string `json:"detail,omitempty"`
}

type operatorRecoveryView struct {
	KillAllCommand    string   `json:"killAllCommand"`
	RestartCommands   []string `json:"restartCommands"`
	LiveCheckCommands []string `json:"liveCheckCommands"`
	Runbook           string   `json:"runbook"`
	Note              string   `json:"note"`
}

func operatorStatusHandler(config Config, surfaceProvider surfaceroute.Provider) http.Handler {
	path := strings.TrimSpace(config.CompositorctlPath)
	if path == "" {
		path = "compositorctl"
	}
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != OperatorStatusPath {
			http.NotFound(response, request)
			return
		}
		if request.Method != http.MethodGet {
			response.Header().Set("Allow", http.MethodGet)
			writeJSON(response, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		ctx, cancel := context.WithTimeout(request.Context(), 3*time.Second)
		defer cancel()
		status := collectOperatorStatus(ctx, path, surfaceProvider, time.Now())
		writeJSON(response, http.StatusOK, status)
	})
}

func collectOperatorStatus(ctx context.Context, compositorctl string, surfaceProvider surfaceroute.Provider, now time.Time) operatorStatusResponse {
	services := []operatorServiceView{
		checkSystemdService(ctx, "agora-de-shellui.service", "user"),
		checkSystemdService(ctx, "agora-de-shell-background.service", "user"),
		checkSystemdService(ctx, "agora-de-shell-panel.service", "user"),
		checkSystemdService(ctx, "agora-wayfire.service", "system"),
		checkSystemdService(ctx, "compositor-bridge.service", "system"),
	}
	sockets := []operatorSocketView{
		checkUnixSocket("/run/agent-os/compositor-control.sock"),
		checkUnixSocket("/run/agent-os/compositor-bridge.sock"),
	}
	surfaces := summarizeSurfaces(ctx, surfaceProvider)
	outputs := listOperatorOutputs(ctx, compositorctl)
	overall := "ok"
	for _, service := range services {
		if service.State != "active" {
			overall = "warn"
		}
	}
	for _, socket := range sockets {
		if socket.State != "available" {
			overall = "warn"
		}
	}
	if surfaces.State != "available" {
		overall = "warn"
	}
	for _, output := range outputs {
		if output.State != "available" {
			overall = "warn"
		}
	}
	return operatorStatusResponse{
		GeneratedAtUnixMillis: now.UnixMilli(),
		Overall:               overall,
		Services:              services,
		Sockets:               sockets,
		Outputs:               outputs,
		Surfaces:              surfaces,
		Recovery: operatorRecoveryView{
			KillAllCommand: "sudo /usr/local/sbin/agora-de-kill-all",
			RestartCommands: []string{
				"systemctl --user restart agora-de-shellui.service",
				"systemctl --user restart agora-de-shell-background.service agora-de-shell-panel.service",
			},
			LiveCheckCommands: []string{
				"./harness/live/check-den-k8.py --systemd-units 'agora-wayfire.service,compositor-bridge.service' --sockets '/run/agent-os/compositor-control.sock,/run/agent-os/compositor-bridge.sock' --shell-url 'http://127.0.0.1:17780/shell/dist/desktop/?surface=dock' --catalog-url 'http://127.0.0.1:17780/api/catalog/apps' --surfaces-url 'http://127.0.0.1:17780/api/surfaces' --work-controls-url 'http://127.0.0.1:17780/api/work-surface-controls' --workspaces-url 'http://127.0.0.1:17780/api/workspaces' --operator-status-url 'http://127.0.0.1:17780/api/operator/status' --surface-app-id io.agorade.ShellPanel --surface-role panel --output-name HDMI-A-1 --output-capture-session den-k8-live --require-capture",
				"./harness/live/check-shell-loop.py --base-url http://127.0.0.1:17780 --output-name HDMI-A-1 --output-capture-session den-k8-shell-loop --require-capture",
			},
			Runbook: "docs/den-k8-visible-shell-runbook.md",
			Note:    "Recovery commands are shown for operator use; the shell does not run privileged recovery actions.",
		},
	}
}

func checkSystemdService(ctx context.Context, name string, scope string) operatorServiceView {
	args := []string{"is-active", name}
	if scope == "user" {
		args = []string{"--user", "is-active", name}
	}
	output, err := exec.CommandContext(ctx, "systemctl", args...).CombinedOutput()
	state := strings.TrimSpace(string(output))
	if state == "" && err != nil {
		state = "unavailable"
	}
	return operatorServiceView{Name: name, Scope: scope, State: state}
}

func checkUnixSocket(path string) operatorSocketView {
	info, err := os.Stat(path)
	if err != nil {
		return operatorSocketView{Path: path, State: "missing"}
	}
	if info.Mode()&os.ModeSocket == 0 {
		return operatorSocketView{Path: path, State: "not_socket"}
	}
	conn, err := net.DialTimeout("unix", path, 300*time.Millisecond)
	if err != nil {
		return operatorSocketView{Path: path, State: "unreachable"}
	}
	_ = conn.Close()
	return operatorSocketView{Path: path, State: "available"}
}

func summarizeSurfaces(ctx context.Context, provider surfaceroute.Provider) operatorSurfaceSummary {
	if provider == nil {
		return operatorSurfaceSummary{State: "unavailable", Detail: "surface provider is not configured"}
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "/api/surfaces", nil)
	if err != nil {
		return operatorSurfaceSummary{State: "unavailable", Detail: err.Error()}
	}
	views, err := provider(request)
	if err != nil {
		return operatorSurfaceSummary{State: "unavailable", Detail: err.Error()}
	}
	summary := operatorSurfaceSummary{State: "available"}
	for _, view := range views {
		if !view.Mapped {
			continue
		}
		summary.Total++
		if view.SurfaceKind == "layer_shell" {
			summary.LayerShell++
		} else {
			summary.Work++
		}
		if view.Focused {
			summary.Focused++
		}
	}
	return summary
}

type compositorctlOutputListResponse struct {
	Outputs []struct {
		Name     string   `json:"name"`
		Mode     string   `json:"mode"`
		Width    int      `json:"width"`
		Height   int      `json:"height"`
		Surfaces []string `json:"surfaces"`
	} `json:"outputs"`
}

func listOperatorOutputs(ctx context.Context, compositorctl string) []operatorOutputView {
	output, err := exec.CommandContext(ctx, compositorctl, "output", "list").CombinedOutput()
	if err != nil {
		detail := strings.TrimSpace(string(output))
		if detail == "" {
			detail = err.Error()
		}
		return []operatorOutputView{{Name: "compositorctl output list", State: "unavailable", Detail: detail}}
	}
	var response compositorctlOutputListResponse
	if err := json.Unmarshal(output, &response); err != nil {
		return []operatorOutputView{{Name: "compositorctl output list", State: "unavailable", Detail: err.Error()}}
	}
	if len(response.Outputs) == 0 {
		return []operatorOutputView{{Name: "outputs", State: "missing", Detail: "no outputs reported"}}
	}
	views := make([]operatorOutputView, 0, len(response.Outputs))
	for _, output := range response.Outputs {
		views = append(views, operatorOutputView{
			Name:         output.Name,
			State:        "available",
			Mode:         output.Mode,
			Width:        output.Width,
			Height:       output.Height,
			SurfaceCount: len(output.Surfaces),
		})
	}
	return views
}

func writeJSON(response http.ResponseWriter, status int, value any) {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(value)
}

func noStore(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		setNoStore(response)
		next.ServeHTTP(response, request)
	})
}

func setNoStore(response http.ResponseWriter) {
	response.Header().Set("Cache-Control", "no-store, max-age=0")
	response.Header().Set("Pragma", "no-cache")
	response.Header().Set("Expires", "0")
}

func shellAssetHandler(root string) http.Handler {
	var resolver staticserve.Resolver
	var hasRoot bool
	if strings.TrimSpace(root) != "" {
		if value, err := staticserve.NewResolver(root); err == nil {
			resolver = value
			hasRoot = true
		}
	}

	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet && request.Method != http.MethodHead {
			response.Header().Set("Allow", "GET, HEAD")
			http.Error(response, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if hasRoot {
			requestPath := strings.TrimPrefix(request.URL.Path, "/shell/dist/")
			resolved, err := resolver.Resolve(requestPath)
			if err != nil {
				http.Error(response, "unsafe shell asset path", http.StatusBadRequest)
				return
			}
			if info, err := os.Stat(resolved); err == nil && !info.IsDir() {
				http.ServeFile(response, request, resolved)
				return
			}
			indexPath := filepath.Join(resolved, "index.html")
			if info, err := os.Stat(indexPath); err == nil && !info.IsDir() {
				http.ServeFile(response, request, indexPath)
				return
			}
		}

		writeShellHTML(response, request)
	})
}

func writeShellHTML(response http.ResponseWriter, request *http.Request) {
	surface := strings.TrimSpace(request.URL.Query().Get("surface"))
	if surface == "" {
		surface = "desktop"
	}
	response.Header().Set("Content-Type", "text/html; charset=utf-8")
	if surface == "dock" || surface == "panel" {
		writePanelHTML(response, surface)
		return
	}
	if surface == "operator" {
		writeOperatorHTML(response)
		return
	}
	writeBackgroundHTML(response, surface, surface == "background-fallback")
}

func writeBackgroundHTML(response http.ResponseWriter, surface string, includeTaskbar bool) {
	escapedSurface := html.EscapeString(surface)
	bodyClass := "background"
	rows := "1fr"
	taskbarHTML := ""
	if includeTaskbar {
		bodyClass = "background with-taskbar"
		rows = "1fr 96px"
		taskbarHTML = `
  <nav class="taskbar" aria-label="Agora DE fallback taskbar">
    <span class="badge">agora-de</span>
    <span class="slot">shell: dock</span>
    <span class="slot">workspace 1</span>
    <span class="slot">ready</span>
  </nav>`
	}
	fmt.Fprintf(response, `<!doctype html>
<html>
<head>
  <title>agora-de shell</title>
  <style>
%s
    html,
    body {
      background: var(--agora-bg);
      color: var(--agora-fg);
      font: var(--agora-font-background);
      height: 100%%;
      margin: 0;
    }
    body {
      box-sizing: border-box;
      display: grid;
      grid-template-rows: %s;
      min-height: 100vh;
    }
    .stage {
      align-items: center;
      display: flex;
      gap: 18px;
      padding: 0 28px;
    }
    .mark {
      background: var(--agora-evidence-accent);
      border-radius: var(--agora-radius-control);
      height: 40px;
      width: 40px;
    }
    .taskbar {
      align-items: center;
      background: var(--agora-surface);
      border-top: 4px solid var(--agora-evidence-accent);
      box-shadow: inset 0 1px 0 var(--agora-border-subtle);
      box-sizing: border-box;
      display: flex;
      gap: 18px;
      min-height: var(--agora-panel-height);
      padding: 0 28px;
    }
    .badge {
      align-items: center;
      background: var(--agora-evidence-strong);
      border-radius: var(--agora-radius-control);
      color: var(--agora-bg);
      display: inline-flex;
      height: var(--agora-control-height);
      justify-content: center;
      min-width: 132px;
      padding: 0 16px;
    }
    .slot {
      align-items: center;
      border: 2px solid var(--agora-border);
      border-radius: var(--agora-radius-control);
      display: inline-flex;
      height: 40px;
      padding: 0 14px;
    }
  </style>
</head>
<body class="%s" data-surface="%s">
  <main class="stage">
    <span class="mark"></span>
    <span>agora-de shell: %s</span>
  </main>%s
</body>
</html>`, theme.MustDefaultTokenCSS(), rows, bodyClass, escapedSurface, escapedSurface, taskbarHTML)
}

func writeOperatorHTML(response http.ResponseWriter) {
	fmt.Fprintf(response, `<!doctype html>
<html>
<head>
  <title>agora-de shell status</title>
  <meta name="color-scheme" content="light">
  <style>
%s
    html,
    body {
      background: var(--agora-bg);
      color: var(--agora-fg);
      font: var(--agora-font-status);
      margin: 0;
      min-height: 100%%;
    }
    body {
      box-sizing: border-box;
      display: grid;
      gap: 24px;
      padding: 32px;
    }
    header,
    section {
      max-width: 1120px;
      width: 100%%;
    }
    header {
      align-items: center;
      display: flex;
      gap: 18px;
    }
    h1,
    h2 {
      font-size: 20px;
      line-height: 1.2;
      margin: 0;
    }
    h2 {
      font-size: 16px;
      margin-bottom: 10px;
    }
    .mark {
      background: var(--agora-evidence-accent);
      border-radius: var(--agora-radius-control);
      height: 36px;
      width: 36px;
    }
    .overall {
      border: 2px solid var(--agora-border);
      border-radius: var(--agora-radius-control);
      margin-left: auto;
      min-width: 96px;
      padding: 10px 14px;
      text-align: center;
    }
    .overall.ok {
      border-color: var(--agora-accent);
    }
    .overall.warn {
      border-color: var(--agora-warning);
    }
    table {
      border-collapse: collapse;
      width: 100%%;
    }
    th,
    td {
      border-bottom: 1px solid var(--agora-border-subtle);
      padding: 9px 8px;
      text-align: left;
      vertical-align: top;
    }
    th {
      color: var(--agora-text-muted);
      font-size: 13px;
      text-transform: uppercase;
    }
    code {
      background: var(--agora-surface-raised);
      border: 1px solid var(--agora-border-subtle);
      border-radius: var(--agora-radius-control);
      display: block;
      font: var(--agora-font-code);
      margin: 8px 0;
      overflow-wrap: anywhere;
      padding: 10px;
    }
    .grid {
      display: grid;
      gap: 18px;
      grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
    }
    .muted {
      color: var(--agora-text-muted);
    }
  </style>
</head>
<body data-surface="operator">
  <header>
    <span class="mark"></span>
    <h1>agora-de shell status</h1>
    <span class="overall warn" id="overall">loading</span>
  </header>
  <section class="grid" aria-label="Status summaries">
    <section>
      <h2>Services</h2>
      <table>
        <thead><tr><th>Name</th><th>Scope</th><th>State</th></tr></thead>
        <tbody id="services"><tr><td colspan="3">loading</td></tr></tbody>
      </table>
    </section>
    <section>
      <h2>Sockets</h2>
      <table>
        <thead><tr><th>Path</th><th>State</th></tr></thead>
        <tbody id="sockets"><tr><td colspan="2">loading</td></tr></tbody>
      </table>
    </section>
  </section>
  <section class="grid" aria-label="Compositor summaries">
    <section>
      <h2>Outputs</h2>
      <table>
        <thead><tr><th>Name</th><th>State</th><th>Mode</th><th>Size</th></tr></thead>
        <tbody id="outputs"><tr><td colspan="4">loading</td></tr></tbody>
      </table>
    </section>
    <section>
      <h2>Surfaces</h2>
      <table>
        <tbody id="surfaces"><tr><td>loading</td></tr></tbody>
      </table>
    </section>
  </section>
  <section aria-label="Recovery">
    <h2>Recovery</h2>
    <div id="recovery"><code>loading</code></div>
  </section>
  <script>
    function cell(value) {
      const td = document.createElement("td");
      td.textContent = String(value || "");
      return td;
    }

    function renderRows(id, values, mapper, columns) {
      const body = document.getElementById(id);
      body.replaceChildren();
      if (!Array.isArray(values) || values.length === 0) {
        const row = document.createElement("tr");
        const empty = cell("none");
        empty.colSpan = columns;
        row.appendChild(empty);
        body.appendChild(row);
        return;
      }
      values.forEach((value) => body.appendChild(mapper(value)));
    }

    function row(...values) {
      const tr = document.createElement("tr");
      values.forEach((value) => tr.appendChild(cell(value)));
      return tr;
    }

    function renderCommands(target, title, commands) {
      if (!Array.isArray(commands) || commands.length === 0) {
        return;
      }
      const label = document.createElement("div");
      label.className = "muted";
      label.textContent = title;
      target.appendChild(label);
      commands.forEach((command) => {
        const code = document.createElement("code");
        code.textContent = command;
        target.appendChild(code);
      });
    }

    async function refresh() {
      const response = await fetch("/api/operator/status", {cache: "no-store"});
      if (!response.ok) {
        throw new Error("operator status returned " + response.status);
      }
      const status = await response.json();
      const overall = document.getElementById("overall");
      overall.textContent = status.overall || "unknown";
      overall.className = "overall " + (status.overall === "ok" ? "ok" : "warn");

      renderRows("services", status.services, (service) => row(service.name, service.scope, service.state), 3);
      renderRows("sockets", status.sockets, (socket) => row(socket.path, socket.state), 2);
      renderRows("outputs", status.outputs, (output) => {
        const size = output.width && output.height ? output.width + "x" + output.height : output.detail || "";
        return row(output.name, output.state, output.mode || "", size);
      }, 4);

      const surfaces = status.surfaces || {};
      const surfaceBody = document.getElementById("surfaces");
      surfaceBody.replaceChildren(
        row("state", surfaces.state || "unknown"),
        row("total", surfaces.total || 0),
        row("layer shell", surfaces.layerShell || 0),
        row("work", surfaces.work || 0),
        row("focused", surfaces.focused || 0)
      );

      const recovery = status.recovery || {};
      const recoveryTarget = document.getElementById("recovery");
      recoveryTarget.replaceChildren();
      renderCommands(recoveryTarget, "Kill all", [recovery.killAllCommand].filter(Boolean));
      renderCommands(recoveryTarget, "Restart", recovery.restartCommands);
      renderCommands(recoveryTarget, "Live checks", recovery.liveCheckCommands);
      if (recovery.runbook) {
        const runbook = document.createElement("code");
        runbook.textContent = recovery.runbook;
        recoveryTarget.appendChild(runbook);
      }
      if (recovery.note) {
        const note = document.createElement("p");
        note.className = "muted";
        note.textContent = recovery.note;
        recoveryTarget.appendChild(note);
      }
    }

    refresh().catch((error) => {
      document.getElementById("overall").textContent = "offline";
      document.getElementById("overall").className = "overall warn";
    });
    setInterval(refresh, 5000);
  </script>
</body>
</html>`, theme.MustDefaultTokenCSS())
}

func writePanelHTML(response http.ResponseWriter, surface string) {
	escapedSurface := html.EscapeString(surface)
	fmt.Fprintf(response, `<!doctype html>
<html>
<head>
  <title>agora-de shell panel</title>
  <meta name="color-scheme" content="light">
  <style>
%s
    html,
    body {
      background: var(--agora-bg) !important;
      color: var(--agora-fg);
      height: 100%%;
      margin: 0;
      overflow: hidden;
      width: 100%%;
    }
    body {
      align-items: stretch;
      box-sizing: border-box;
      display: flex;
      font: var(--agora-font-panel);
    }
    .panel {
      align-items: center;
      background: var(--agora-surface);
      border-top: 4px solid var(--agora-evidence-accent);
      box-shadow: inset 0 1px 0 var(--agora-border-subtle);
      box-sizing: border-box;
      display: flex;
      gap: var(--agora-panel-gap);
      min-height: var(--agora-panel-height);
      padding: 0 var(--agora-panel-padding-x);
      width: 100vw;
    }
    button {
      font: inherit;
    }
    button:disabled {
      opacity: 0.55;
    }
    .brand,
    .control,
    .workspace,
    .status,
    .clock {
      align-items: center;
      border-radius: var(--agora-radius-control);
      display: inline-flex;
      height: var(--agora-control-height);
      justify-content: center;
      padding: 0 16px;
      white-space: nowrap;
    }
    .brand {
      background: var(--agora-evidence-strong);
      color: var(--agora-bg);
      min-width: 132px;
    }
    .control {
      background: var(--agora-accent);
      border: 0;
      color: var(--agora-fg);
      min-width: 94px;
    }
    .control.secondary {
      background: var(--agora-surface-raised);
      border: 2px solid var(--agora-border);
    }
    .app-search {
      background: var(--agora-surface-raised);
      border: 2px solid var(--agora-border);
      border-radius: var(--agora-radius-control);
      box-sizing: border-box;
      color: var(--agora-fg);
      font: inherit;
      height: var(--agora-control-height);
      min-width: 124px;
      padding: 0 12px;
      width: 124px;
    }
    .workspace,
    .status,
    .clock {
      border: 2px solid var(--agora-border);
      color: var(--agora-fg);
    }
    button.workspace {
      background: var(--agora-surface-raised);
      font: inherit;
    }
    .dock-section {
      align-items: center;
      display: flex;
      gap: 10px;
      min-width: 0;
    }
    .apps {
      flex: 0 1 680px;
      overflow: hidden;
    }
    .apps.expanded {
      flex: 1 1 860px;
    }
    .panel.apps-open .apps {
      flex: 1 1 auto;
    }
    .app-list {
      align-items: center;
      display: flex;
      gap: 10px;
      min-width: 0;
      overflow: hidden;
    }
    .apps.expanded .app-list,
    .panel.apps-open .app-list {
      overflow-x: auto;
      padding-bottom: 2px;
    }
    .running {
      flex: 1 1 auto;
      overflow: hidden;
    }
    .panel.apps-open .running {
      display: none;
    }
    .dock-item {
      align-items: center;
      background: var(--agora-surface-raised);
      border: 2px solid var(--agora-border-subtle);
      border-radius: var(--agora-radius-control);
      color: var(--agora-fg);
      display: inline-flex;
      height: var(--agora-control-height);
      max-width: 180px;
      min-width: 86px;
      overflow: hidden;
      padding: 0 12px;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
    button.dock-item {
      cursor: pointer;
    }
    .dock-item.app-item {
      align-items: center;
      gap: 8px;
      justify-content: center;
      line-height: 1.05;
      max-width: 220px;
    }
    .app-icon {
      align-items: center;
      background: var(--agora-evidence-strong);
      border-radius: var(--agora-radius-control);
      color: var(--agora-bg);
      display: inline-flex;
      flex: 0 0 auto;
      font-size: 13px;
      height: 26px;
      justify-content: center;
      width: 26px;
    }
    .app-copy {
      display: block;
      min-width: 0;
    }
    .app-name,
    .app-meta,
    .app-reason {
      display: block;
      max-width: 100%%;
      overflow: hidden;
      text-overflow: ellipsis;
    }
    .app-meta,
    .app-reason {
      color: var(--agora-text-muted);
      font-size: 12px;
      margin-top: 3px;
    }
    .dock-item.disabled {
      border-color: var(--agora-border);
    }
    .dock-item.focused {
      border-color: var(--agora-accent);
      box-shadow: inset 0 -3px 0 var(--agora-accent);
    }
    .surface-actions {
      align-items: center;
      display: inline-flex;
      gap: 6px;
    }
    .surface-action {
      background: var(--agora-surface-raised);
      border: 2px solid var(--agora-border);
      border-radius: var(--agora-radius-control);
      color: var(--agora-fg);
      height: var(--agora-control-height);
      min-width: 58px;
      padding: 0 10px;
    }
    .spacer {
      flex: 1 1 auto;
      min-width: 24px;
    }
    .status {
      min-width: 88px;
    }
    .status.ready {
      border-color: var(--agora-accent);
    }
    .status.warn {
      border-color: var(--agora-warning);
    }
    .muted {
      color: var(--agora-text-muted);
    }
  </style>
</head>
<body data-surface="%s">
  <main class="panel" aria-label="Agora DE shell panel">
    <span class="brand">agora-de</span>
    <button class="control" id="apps-button" type="button" aria-pressed="false">Apps</button>
    <button class="control secondary" id="refresh-button" type="button">Refresh</button>
    <button class="control secondary" id="operator-button" type="button">Status</button>
    <section class="dock-section apps" id="apps-section" aria-label="Applications">
      <input class="app-search" id="app-search" type="search" aria-label="Search apps" placeholder="Search">
      <span class="app-list" id="apps-list">
        <span class="dock-item muted">loading apps</span>
      </span>
    </section>
    <section class="dock-section running" id="running-list" aria-label="Running surfaces">
      <span class="dock-item muted">loading surfaces</span>
    </section>
    <button class="workspace" id="workspace-label" type="button">workspace 1</button>
    <span class="status" id="status-label">starting</span>
    <time class="clock" id="clock-label">--:--</time>
  </main>
  <script>
    const state = {
      apps: [],
      surfaces: [],
      workspace: {id: "workspace-1", name: "workspace 1", active: true, surfaceCount: 0},
      surface: %q,
      appQuery: "",
      appsExpanded: false
    };

    function text(value, fallback) {
      const trimmed = String(value || "").trim();
      return trimmed || fallback;
    }

    function item(label, className) {
      const element = document.createElement("span");
      element.className = "dock-item" + (className ? " " + className : "");
      element.textContent = label;
      element.title = label;
      return element;
    }

    function button(label, className, onClick) {
      const element = document.createElement("button");
      element.type = "button";
      element.className = className;
      element.textContent = label;
      element.title = label;
      element.addEventListener("click", onClick);
      return element;
    }

    function renderList(id, emptyLabel, values, mapper, limit) {
      const target = document.getElementById(id);
      target.replaceChildren();
      if (!values.length) {
        target.appendChild(item(emptyLabel, "muted"));
        return;
      }
      values.slice(0, limit || 4).forEach((value) => target.appendChild(mapper(value)));
    }

    function renderApp(app) {
      const label = text(app.name, app.id);
      const reason = text(app.disabledReason, app.launchable ? "" : "not launchable");
      const element = document.createElement("button");
      element.type = "button";
      element.className = "dock-item app-item" + (app.launchable ? "" : " disabled");
      element.disabled = !app.launchable;
      element.title = reason ? label + " - " + reason : label;
      element.addEventListener("click", () => launchApp(app.id));

      const name = document.createElement("span");
      name.className = "app-name";
      name.textContent = label;
      const icon = document.createElement("span");
      icon.className = "app-icon";
      icon.textContent = text(app.iconLabel, label.slice(0, 1).toUpperCase());
      icon.title = text(app.iconRef, text(app.icon, ""));
      const copy = document.createElement("span");
      copy.className = "app-copy";
      copy.appendChild(name);
      if (reason) {
        const detail = document.createElement("span");
        detail.className = "app-reason";
        detail.textContent = reason;
        copy.appendChild(detail);
      } else {
        const category = document.createElement("span");
        category.className = "app-meta";
        category.textContent = text(app.category, "Other");
        copy.appendChild(category);
      }
      element.appendChild(icon);
      element.appendChild(copy);
      return element;
    }

    function render() {
      document.querySelector(".panel").className = "panel" + (state.appsExpanded ? " apps-open" : "");
      document.getElementById("apps-section").className = "dock-section apps" + (state.appsExpanded ? " expanded" : "");
      const query = state.appQuery.trim().toLowerCase();
      const apps = query
        ? state.apps.filter((app) => (text(app.name, app.id) + " " + text(app.id, "") + " " + text(app.category, "")).toLowerCase().includes(query))
        : state.apps;
      const appsButton = document.getElementById("apps-button");
      appsButton.textContent = state.appsExpanded ? "Hide Apps" : "Apps";
      appsButton.title = state.appsExpanded ? "Hide applications" : state.apps.length + " apps";
      appsButton.setAttribute("aria-pressed", state.appsExpanded ? "true" : "false");
      renderList("apps-list", query ? "no matches" : "no apps", apps, renderApp, state.appsExpanded ? 12 : 4);
      const workSurfaces = state.surfaces.filter((surface) => surface.mapped && surface.surfaceKind !== "layer_shell");
      renderList("running-list", "no running apps", workSurfaces, (surface) => {
        const group = document.createElement("span");
        group.className = "surface-actions";
        const label = text(surface.title, text(surface.appId, surface.id));
        group.appendChild(button(label, "dock-item" + (surface.focused ? " focused" : ""), () => actOnSurface(surface.id, "focus")));
        group.appendChild(button("Close", "surface-action", () => actOnSurface(surface.id, "close")));
        return group;
      });
      const status = document.getElementById("status-label");
      if (state.appsExpanded) {
        status.textContent = apps.length + " apps";
        status.className = "status ready";
      } else {
        status.textContent = workSurfaces.length ? workSurfaces.length + " running" : "ready";
        status.className = "status " + (workSurfaces.length ? "ready" : "warn");
      }
      const workspace = document.getElementById("workspace-label");
      workspace.textContent = text(state.workspace.name, "workspace 1");
      workspace.title = state.workspace.surfaceCount ? state.workspace.surfaceCount + " work surfaces" : "workspace 1";
    }

    async function loadJSON(path) {
      const response = await fetch(path, {cache: "no-store"});
      if (!response.ok) {
        throw new Error(path + " returned " + response.status);
      }
      return response.json();
    }

    async function refresh() {
      try {
        const [catalog, surfaces, workspaces] = await Promise.all([
          loadJSON("/api/catalog/apps"),
          loadJSON("/api/surfaces"),
          loadJSON("/api/workspaces")
        ]);
        state.apps = Array.isArray(catalog.apps) ? catalog.apps : [];
        state.surfaces = Array.isArray(surfaces.surfaces) ? surfaces.surfaces : [];
        if (Array.isArray(workspaces.workspaces) && workspaces.workspaces.length) {
          state.workspace = workspaces.workspaces.find((workspace) => workspace.active) || workspaces.workspaces[0];
        }
        render();
      } catch (error) {
        const status = document.getElementById("status-label");
        status.textContent = "offline";
        status.className = "status warn";
      }
    }

    async function postJSON(path, body) {
      const response = await fetch(path, {
        method: "POST",
        headers: {"Content-Type": "application/json"},
        body: JSON.stringify(body)
      });
      if (!response.ok) {
        throw new Error(path + " returned " + response.status);
      }
      return response.json();
    }

    async function launchApp(appId) {
      const status = document.getElementById("status-label");
      status.textContent = "launching";
      status.className = "status ready";
      try {
        await postJSON("/api/catalog/launch", {appId});
        await refresh();
      } catch (error) {
        status.textContent = "launch failed";
        status.className = "status warn";
      }
    }

    async function actOnSurface(surfaceId, action) {
      const status = document.getElementById("status-label");
      status.textContent = action;
      status.className = "status ready";
      try {
        await postJSON("/api/surfaces/action", {surfaceId, action});
        await refresh();
      } catch (error) {
        status.textContent = action + " failed";
        status.className = "status warn";
      }
    }

    async function activateWorkspace() {
      const status = document.getElementById("status-label");
      status.textContent = "workspace";
      status.className = "status ready";
      try {
        await postJSON("/api/workspaces/action", {workspaceId: "workspace-1", action: "activate"});
        await refresh();
      } catch (error) {
        status.textContent = "workspace failed";
        status.className = "status warn";
      }
    }

    function toggleApps() {
      state.appsExpanded = !state.appsExpanded;
      render();
      if (state.appsExpanded) {
        document.getElementById("app-search").focus();
      }
    }

    function updateClock() {
      const now = new Date();
      document.getElementById("clock-label").textContent = now.toLocaleTimeString([], {
        hour: "2-digit",
        minute: "2-digit"
      });
    }

    document.getElementById("apps-button").addEventListener("click", toggleApps);
    document.getElementById("app-search").addEventListener("input", (event) => {
      state.appQuery = event.target.value;
      state.appsExpanded = true;
      render();
    });
    document.getElementById("refresh-button").addEventListener("click", refresh);
    document.getElementById("operator-button").addEventListener("click", () => launchApp("shell-status"));
    document.getElementById("workspace-label").addEventListener("click", activateWorkspace);
    updateClock();
    refresh();
    setInterval(updateClock, 30000);
    setInterval(refresh, 3000);
  </script>
</body>
</html>`, theme.MustDefaultTokenCSS(), escapedSurface, escapedSurface)
}
