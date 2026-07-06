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
	"syscall"
	"time"

	"agora-de.local/go/internal/appcatalog"
	"agora-de.local/go/internal/nativelaunch"
	"agora-de.local/go/internal/session"
	"agora-de.local/go/internal/shellui/catalog"
	"agora-de.local/go/internal/shellui/catalogroute"
	"agora-de.local/go/internal/shellui/layoutroute"
	"agora-de.local/go/internal/shellui/staticserve"
	"agora-de.local/go/internal/shellui/surfaceroute"
	"agora-de.local/go/internal/shellui/surfaces"
	"agora-de.local/go/internal/shellui/theme"
)

const (
	DefaultListenAddress = "127.0.0.1:7780"
	LayoutPath           = layoutroute.LayoutPath
	LayoutActionPath     = layoutroute.ActionPath
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
	NativeLaunchAllowAll                        = "*"

	NativeDisabledCodeProviderDisabled = "native_launch_disabled"
	NativeDisabledCodeNotAllowlisted   = "native_launch_not_allowlisted"
	NativeDisabledCodeUnavailable      = "native_launch_unavailable"
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
	mux.Handle(LayoutPath, layoutroute.New(layoutroute.Config{
		CompositorctlPath: config.CompositorctlPath,
		UseCompositorctl:  strings.TrimSpace(config.SurfaceProvider) == SurfaceProviderCompositorctl,
		SurfaceProvider:   surfaceProvider,
	}))
	mux.Handle(LayoutActionPath, layoutroute.NewAction(layoutroute.Config{CompositorctlPath: config.CompositorctlPath}))
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
		allowlisted := nativeLaunchAllowed(nativeAllowlist, views[index].ID)
		launchable := ok &&
			nativeMode == NativeLaunchProviderStructuredCompositorctl &&
			allowlisted &&
			nativelaunch.CanPrepare(entry)
		views[index].Launchable = launchable
		if !launchable {
			disabled := nativeDisabledState(nativeMode, allowlisted, entry)
			views[index].DisabledCode = disabled.Code
			views[index].DisabledReason = disabled.Reason
		}
	}
	return views, nil
}

func nativeLaunchAllowed(allowlist map[string]bool, appID string) bool {
	return allowlist[NativeLaunchAllowAll] || allowlist[appID]
}

type nativeDisabled struct {
	Code   string
	Reason string
}

func nativeDisabledState(mode string, allowlisted bool, entry appcatalog.Entry) nativeDisabled {
	switch {
	case mode == NativeLaunchProviderDisabled:
		return nativeDisabled{Code: NativeDisabledCodeProviderDisabled, Reason: "native launch disabled"}
	case !nativelaunch.CanPrepare(entry):
		return nativeDisabled{Code: catalog.DisabledCodeUnsupportedDesktopEntry, Reason: "unsupported desktop entry"}
	case !allowlisted:
		return nativeDisabled{Code: NativeDisabledCodeNotAllowlisted, Reason: "not enabled for native launch"}
	default:
		return nativeDisabled{Code: NativeDisabledCodeUnavailable, Reason: "not launchable"}
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
	URL           string
	Title         string
	AppID         string
	LayerShell    bool
	LayerRole     string
	Width         int
	Height        int
	ExclusiveZone int
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
		"shell-launcher": {
			URL:           "http://127.0.0.1:17780/shell/dist/desktop/?surface=launcher",
			Title:         "Agora DE App Launcher",
			AppID:         "io.agorade.ShellLauncher",
			LayerShell:    true,
			LayerRole:     "overlay",
			Width:         2560,
			Height:        1440,
			ExclusiveZone: 0,
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
		if nativeMode != NativeLaunchProviderStructuredCompositorctl || !nativeLaunchAllowed(nativeAllowlist, launch.AppID) {
			return catalogroute.LaunchResult{}, fmt.Errorf("app %q is not launchable by shellui", launch.AppID)
		}
		return launchNativeTarget(request, config, path, launch.AppID, entry)
	}
}

func launchWebviewTarget(request *http.Request, path string, appID string, target launchTarget) (catalogroute.LaunchResult, error) {
	ctx, cancel := context.WithTimeout(request.Context(), 8*time.Second)
	defer cancel()
	if target.LayerShell {
		return launchLayerShellWebviewTarget(ctx, path, request.Host, appID, target)
	}
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

func launchLayerShellWebviewTarget(ctx context.Context, path string, host string, appID string, target launchTarget) (catalogroute.LaunchResult, error) {
	url := target.URL
	if strings.Contains(url, "127.0.0.1:17780") && host != "" {
		url = strings.Replace(url, "127.0.0.1:17780", host, 1)
	}
	width := target.Width
	if width <= 0 {
		width = 1280
	}
	height := target.Height
	if height <= 0 {
		height = 800
	}
	exclusiveZone := target.ExclusiveZone
	output, err := exec.CommandContext(ctx, path,
		"launch",
		"--arg", "/usr/bin/env",
		"--arg", "GDK_BACKEND=wayland",
		"--arg", "LD_PRELOAD=/usr/lib/libgtk4-layer-shell.so",
		"--arg", "/usr/bin/python3",
		"--arg", "/home/agent/.local/bin/agora-de-gtk4-layer-shell-webview",
		"--arg", "--url", "--arg", url,
		"--arg", "--role", "--arg", firstNonEmpty(target.LayerRole, "overlay"),
		"--arg", "--width", "--arg", fmt.Sprint(width),
		"--arg", "--height", "--arg", fmt.Sprint(height),
		"--arg", "--exclusive-zone", "--arg", fmt.Sprint(exclusiveZone),
		"--arg", "--title", "--arg", target.Title,
		"--arg", "--app-id", "--arg", target.AppID,
		"--expected-app-id", target.AppID,
		"--wait-surface",
		"--wait-timeout-ms", "5000",
		"--session-token", "shellui-webview",
		"--audit-correlation-id", "shellui:"+appID,
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
		ID          string                 `json:"id"`
		Label       string                 `json:"label"`
		AppID       string                 `json:"app_id"`
		Title       string                 `json:"title"`
		Role        string                 `json:"role"`
		SurfaceKind string                 `json:"surface_kind"`
		Visible     bool                   `json:"visible"`
		OutputID    string                 `json:"output_id"`
		WorkspaceID string                 `json:"workspace_id"`
		ZoneID      string                 `json:"zone_id"`
		LayoutMode  string                 `json:"layout_mode"`
		LayoutRole  string                 `json:"layout_role"`
		Geometry    *surfaces.GeometryView `json:"geometry"`
	} `json:"surface"`
	Client struct {
		PID int `json:"pid"`
		UID int `json:"uid"`
	} `json:"client"`
	LaunchID           string                 `json:"launch_id"`
	LastEvent          string                 `json:"last_event"`
	Focused            bool                   `json:"focused"`
	Visible            bool                   `json:"visible"`
	OutputID           string                 `json:"output_id"`
	WorkspaceID        string                 `json:"workspace_id"`
	ZoneID             string                 `json:"zone_id"`
	LayoutMode         string                 `json:"layout_mode"`
	LayoutRole         string                 `json:"layout_role"`
	Geometry           *surfaces.GeometryView `json:"geometry"`
	FrameCount         int                    `json:"frame_count"`
	ContentCommitCount int                    `json:"content_commit_count"`
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
			Label:              tracked.Surface.Label,
			AppID:              tracked.Surface.AppID,
			Title:              tracked.Surface.Title,
			Role:               tracked.Surface.Role,
			SurfaceKind:        tracked.Surface.SurfaceKind,
			LaunchID:           tracked.LaunchID,
			OwnerUID:           tracked.Client.UID,
			Mapped:             mapped,
			Focused:            tracked.Focused,
			Visible:            tracked.Visible || tracked.Surface.Visible,
			OutputID:           firstNonEmpty(tracked.OutputID, tracked.Surface.OutputID),
			WorkspaceID:        firstNonEmpty(tracked.WorkspaceID, tracked.Surface.WorkspaceID),
			ZoneID:             firstNonEmpty(tracked.ZoneID, tracked.Surface.ZoneID),
			LayoutMode:         firstNonEmpty(tracked.LayoutMode, tracked.Surface.LayoutMode),
			LayoutRole:         firstNonEmpty(tracked.LayoutRole, tracked.Surface.LayoutRole),
			Geometry:           firstGeometryView(tracked.Geometry, tracked.Surface.Geometry),
			InputDeniedCount:   0,
			FrameCount:         tracked.FrameCount,
			ContentCommitCount: tracked.ContentCommitCount,
		})
	}
	return views, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func firstGeometryView(values ...*surfaces.GeometryView) *surfaces.GeometryView {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func writeCompositorctlError(response http.ResponseWriter, output []byte, err error) {
	message := strings.TrimSpace(string(output))
	if message == "" && err != nil {
		message = err.Error()
	}
	errorClass, cleanMessage := parseCompositorctlError(message)
	status := http.StatusServiceUnavailable
	if errorClass == "backend_unsupported" {
		status = http.StatusNotImplemented
	}
	writeJSON(response, status, classifiedAPIError{Error: cleanMessage, ErrorClass: errorClass})
}

func parseCompositorctlError(message string) (string, string) {
	message = strings.TrimSpace(message)
	const prefix = "server["
	if start := strings.Index(message, prefix); start >= 0 {
		rest := strings.TrimPrefix(message[start:], prefix)
		if end := strings.Index(rest, "]"); end > 0 {
			errorClass := rest[:end]
			clean := strings.TrimSpace(strings.TrimPrefix(rest[end+1:], ":"))
			if clean == "" {
				clean = message
			}
			return errorClass, clean
		}
	}
	return "", message
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

type classifiedAPIError struct {
	Error      string `json:"error"`
	ErrorClass string `json:"errorClass,omitempty"`
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
		if action.Action == "close" {
			closed, err := closeShellLayerSurface(ctx, path, action.SurfaceID)
			if err != nil {
				writeJSON(response, http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
				return
			}
			if closed {
				writeJSON(response, http.StatusAccepted, surfaceActionResponse{
					Action:    action.Action,
					SurfaceID: action.SurfaceID,
					Status:    "accepted",
				})
				return
			}
		}
		if output, err := exec.CommandContext(ctx, path, args...).CombinedOutput(); err != nil {
			writeCompositorctlError(response, output, err)
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
	case "maximize":
		return []string{"surface", "maximize", "--surface", action.SurfaceID, "--enabled=true", "--timeout-ms", "2000"}, true
	case "minimize":
		return []string{"surface", "minimize", "--surface", action.SurfaceID, "--enabled=true", "--timeout-ms", "2000"}, true
	case "fullscreen":
		return []string{"surface", "fullscreen", "--surface", action.SurfaceID, "--enabled=true", "--timeout-ms", "2000"}, true
	case "setFloating":
		return []string{"surface", "set-floating", "--surface", action.SurfaceID, "--enabled=true", "--timeout-ms", "2000"}, true
	default:
		return nil, false
	}
}

var signalProcess = syscall.Kill

func closeShellLayerSurface(ctx context.Context, compositorctlPath string, surfaceID string) (bool, error) {
	output, err := exec.CommandContext(ctx, compositorctlPath, "list-surfaces").Output()
	if err != nil {
		return false, fmt.Errorf("compositorctl list-surfaces: %w", err)
	}
	var response compositorctlListSurfacesResponse
	if err := json.Unmarshal(output, &response); err != nil {
		return false, fmt.Errorf("decode compositorctl surfaces: %w", err)
	}
	for _, tracked := range response.Surfaces {
		if tracked.Surface.ID != surfaceID {
			continue
		}
		if tracked.Surface.SurfaceKind != "layer_shell" || !isCloseableShellLayerApp(tracked.Surface.AppID) {
			return false, nil
		}
		if tracked.Client.PID <= 0 {
			return true, nil
		}
		if err := signalProcess(tracked.Client.PID, syscall.SIGTERM); err != nil && processExists(tracked.Client.PID) {
			return true, fmt.Errorf("terminate shell layer client %d: %w", tracked.Client.PID, err)
		}
		return true, nil
	}
	return false, nil
}

func isCloseableShellLayerApp(appID string) bool {
	switch strings.TrimSpace(appID) {
	case "io.agorade.ShellLauncher", "io.agorade.ShellOverlay":
		return true
	default:
		return false
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
				"./harness/live/check-native-launch.py --base-url http://127.0.0.1:17780 --output-name HDMI-A-1 --output-capture-session den-k8-native-launch --require-capture",
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
	if surface == "launcher" {
		writeLauncherHTML(response)
		return
	}
	if surface == "operator" {
		writeOperatorHTML(response)
		return
	}
	if surface == "overlay" {
		writeOverlayHTML(response)
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

func writeOverlayHTML(response http.ResponseWriter) {
	fmt.Fprintf(response, `<!doctype html>
<html>
<head>
  <title>agora-de agent overlay</title>
  <meta name="color-scheme" content="dark">
  <style>
%s
    html,
    body {
      background: transparent !important;
      color: var(--agora-fg);
      height: 100%%;
      margin: 0;
      overflow: hidden;
      width: 100%%;
    }
    body {
      font: var(--agora-font-status);
      pointer-events: none;
    }
    .agent-overlay {
      height: 100vh;
      inset: 0;
      overflow: hidden;
      pointer-events: none;
      position: fixed;
      width: 100vw;
    }
    .zone-hints {
      display: grid;
      grid-template-columns: 1fr 1fr;
      height: calc(100vh - var(--agora-panel-height));
      inset: 0 0 var(--agora-panel-height) 0;
      opacity: 0.14;
      position: absolute;
    }
    .zone-hint {
      border: 2px dashed var(--agora-border);
      box-sizing: border-box;
      color: var(--agora-text-muted);
      padding: 12px;
      text-transform: uppercase;
    }
    .window-box {
      border: 3px solid var(--agora-evidence-accent);
      box-shadow:
        0 0 0 1px var(--agora-surface-strong),
        inset 0 0 0 1px var(--agora-surface-strong);
      box-sizing: border-box;
      min-height: 72px;
      min-width: 120px;
      position: absolute;
    }
    .window-box.focused {
      border-color: var(--agora-warning);
      box-shadow:
        0 0 0 2px var(--agora-surface-strong),
        0 0 22px rgba(251, 191, 36, 0.75),
        inset 0 0 0 2px var(--agora-warning);
    }
    .label {
      align-items: center;
      background: rgba(8, 13, 30, 0.72);
      border: 2px solid var(--agora-evidence-accent);
      color: var(--agora-fg);
      display: inline-flex;
      gap: 8px;
      left: auto;
      max-width: calc(100%% - 16px);
      min-height: 36px;
      padding: 0 10px;
      position: absolute;
      right: 8px;
      top: 8px;
      white-space: nowrap;
    }
    .focused .label {
      border-color: var(--agora-warning);
    }
    .number {
      align-items: center;
      background: var(--agora-evidence-accent);
      color: var(--agora-surface-strong);
      display: inline-flex;
      font: var(--agora-font-code);
      height: 24px;
      justify-content: center;
      min-width: 24px;
      padding: 0 4px;
    }
    .copy {
      display: block;
      max-width: 320px;
      overflow: hidden;
      text-overflow: ellipsis;
    }
    .bounds {
      background: rgba(8, 13, 30, 0.62);
      border: 2px solid var(--agora-border);
      bottom: 8px;
      color: var(--agora-text-muted);
      font: var(--agora-font-code);
      left: 8px;
      padding: 5px 8px;
      position: absolute;
    }
    .meta {
      align-items: center;
      bottom: 8px;
      display: flex;
      flex-wrap: wrap;
      gap: 6px;
      max-width: calc(100%% - 16px);
      opacity: 0.62;
      position: absolute;
      right: 8px;
    }
    .chip {
      background: rgba(8, 13, 30, 0.62);
      border: 2px solid var(--agora-border);
      color: var(--agora-fg);
      font: var(--agora-font-code);
      padding: 5px 7px;
    }
    .action-hints {
      border-color: var(--agora-evidence-accent);
      color: var(--agora-text-muted);
    }
    .fallback-stack {
      display: grid;
      gap: 10px;
      left: 24px;
      max-width: 520px;
      position: absolute;
      top: 24px;
    }
    .fallback-item {
      background: var(--agora-surface-strong);
      border: 2px solid var(--agora-evidence-accent);
      color: var(--agora-fg);
      padding: 10px 12px;
    }
    .empty {
      background: var(--agora-surface-strong);
      border: 2px solid var(--agora-border);
      color: var(--agora-text-muted);
      left: 24px;
      padding: 10px 12px;
      position: absolute;
      top: 24px;
    }
  </style>
</head>
<body data-surface="overlay">
  <main class="agent-overlay" id="agent-overlay-surface" aria-label="Agent-visible window labels and bounds overlay">
    <section class="zone-hints" id="zone-hints" aria-label="Workspace zone hints">
      <span class="zone-hint">primary</span>
      <span class="zone-hint">secondary</span>
    </section>
    <section id="overlay-labels" aria-label="Surface labels"></section>
  </main>
  <script>
    const state = {
      layout: {mode: "freeform", revision: 0, surfaces: [], workspaces: []},
      surfaces: []
    };

    function text(value, fallback) {
      const trimmed = String(value || "").trim();
      return trimmed || fallback;
    }

    function number(value, fallback) {
      const parsed = Number(value);
      return Number.isFinite(parsed) ? parsed : fallback;
    }

    function geometryFor(surface) {
      const geometry = surface && surface.geometry;
      if (!geometry || typeof geometry !== "object") {
        return null;
      }
      const x = number(geometry.x, 0);
      const y = number(geometry.y, 0);
      const width = number(geometry.width, 0);
      const height = number(geometry.height, 0);
      if (width <= 0 || height <= 0) {
        return null;
      }
      return {x, y, width, height};
    }

    function surfaceName(surface) {
      return text(surface.title, text(surface.appId, surface.surfaceId));
    }

    function activeWorkspace() {
      const workspaces = Array.isArray(state.layout.workspaces) ? state.layout.workspaces : [];
      return workspaces.find((workspace) => workspace.active) || workspaces[0] || {zones: []};
    }

    function layoutRuleLabel() {
      const settings = state.layout.settings || {};
      return text(settings.rule, text(state.layout.mode, "freeform"));
    }

    function renderZoneHints() {
      const target = document.getElementById("zone-hints");
      const zones = Array.isArray(activeWorkspace().zones) ? activeWorkspace().zones : [];
      const zoneLabels = zones.length ? zones.map((zone) => text(zone.id, "zone")) : ["primary", "secondary"];
      target.replaceChildren();
      target.style.gridTemplateColumns = "repeat(" + Math.max(1, zoneLabels.length) + ", 1fr)";
      zoneLabels.forEach((zoneId) => {
        const hint = document.createElement("span");
        hint.className = "zone-hint";
        hint.textContent = zoneId;
        target.appendChild(hint);
      });
    }

    function actionHints(surface) {
      const participation = text(surface.participation, surface.floating ? "floating" : "tiled");
      return participation === "floating" || participation === "transient"
        ? "focus close tile"
        : "focus close float zone reset";
    }

    function chip(value, className) {
      const element = document.createElement("span");
      element.className = "chip" + (className ? " " + className : "");
      element.textContent = value;
      return element;
    }

    function renderBox(surface) {
      const geometry = geometryFor(surface);
      if (!geometry) {
        return null;
      }
      const element = document.createElement("article");
      element.className = "window-box" + (surface.focused ? " focused" : "");
      element.dataset.surfaceId = surface.surfaceId;
      element.dataset.zoneId = text(surface.zoneId, "primary");
      element.dataset.order = String(number(surface.order, 0));
      element.dataset.participation = text(surface.participation, surface.floating ? "floating" : "tiled");
      element.dataset.layoutRule = layoutRuleLabel();
      element.style.left = Math.max(0, geometry.x) + "px";
      element.style.top = Math.max(0, geometry.y) + "px";
      element.style.width = geometry.width + "px";
      element.style.height = geometry.height + "px";

      const label = document.createElement("span");
      label.className = "label";
      const numberBadge = document.createElement("span");
      numberBadge.className = "number";
      numberBadge.textContent = text(surface.label, String(number(surface.order, 0) + 1));
      const copy = document.createElement("span");
      copy.className = "copy";
      copy.textContent = "#" + text(surface.surfaceId, "?") + " / " + surfaceName(surface);
      label.append(numberBadge, copy);

      const bounds = document.createElement("span");
      bounds.className = "bounds";
      bounds.textContent = geometry.x + "," + geometry.y + " " + geometry.width + "x" + geometry.height;
      const meta = document.createElement("span");
      meta.className = "meta";
      meta.append(
        chip("order " + String(number(surface.order, 0))),
        chip(text(surface.zoneId, "primary")),
        chip(text(surface.participation, surface.floating ? "floating" : "tiled")),
        chip(layoutRuleLabel()),
        chip(surface.focused ? "focused" : "unfocused"),
        chip(actionHints(surface), "action-hints")
      );
      element.append(label, bounds, meta);
      return element;
    }

    function renderFallback(surfaces) {
      const stack = document.createElement("section");
      stack.className = "fallback-stack";
      surfaces.forEach((surface, index) => {
        const item = document.createElement("span");
        item.className = "fallback-item";
        item.textContent = text(surface.label, String(index + 1)) + " / " + surfaceName(surface) + " / " + text(surface.zoneId, "primary");
        stack.appendChild(item);
      });
      return stack;
    }

    function render() {
      const target = document.getElementById("overlay-labels");
      target.replaceChildren();
      renderZoneHints();
      const surfaces = (Array.isArray(state.layout.surfaces) ? state.layout.surfaces : [])
        .filter((surface) => surface && surface.visible !== false);
      if (!surfaces.length) {
        const empty = document.createElement("span");
        empty.className = "empty";
        empty.textContent = "no work surfaces";
        target.appendChild(empty);
        return;
      }
      const boxes = surfaces.map(renderBox).filter(Boolean);
      if (boxes.length) {
        boxes.forEach((box) => target.appendChild(box));
        return;
      }
      target.appendChild(renderFallback(surfaces));
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
        const [layout, surfaces] = await Promise.all([
          loadJSON("/api/layout"),
          loadJSON("/api/surfaces")
        ]);
        state.layout = layout.layout || state.layout;
        state.surfaces = Array.isArray(surfaces.surfaces) ? surfaces.surfaces : [];
        render();
      } catch (error) {
        render();
      }
    }

    refresh();
    setInterval(refresh, 1000);
  </script>
</body>
</html>`, theme.MustDefaultTokenCSS())
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

func writeLauncherHTML(response http.ResponseWriter) {
	fmt.Fprintf(response, `<!doctype html>
<html>
<head>
  <title>agora-de app launcher</title>
  <meta name="color-scheme" content="dark">
  <style>
%s
    html,
    body {
      background: var(--agora-bg);
      color: var(--agora-fg);
      font: var(--agora-font-status);
      height: 100%%;
      margin: 0;
      overflow: hidden;
      width: 100%%;
    }
    body {
      box-sizing: border-box;
      display: block;
      height: 100vh;
      min-height: 0;
    }
    .launcher {
      background: var(--agora-surface);
      border: 2px solid var(--agora-border);
      border-bottom-color: var(--agora-accent);
      border-radius: var(--agora-radius-control);
      box-shadow: 0 18px 60px rgba(0, 0, 0, 0.42);
      bottom: calc(var(--agora-panel-height) + 10px);
      display: grid;
      grid-template-rows: auto 1fr auto;
      height: min(600px, calc(100vh - var(--agora-panel-height) - 24px));
      left: var(--agora-panel-padding-x);
      min-height: 0;
      overflow: hidden;
      position: fixed;
      width: min(760px, calc(100vw - 44px));
    }
    .launcher-header {
      align-items: center;
      border-bottom: 1px solid var(--agora-border-subtle);
      display: flex;
      gap: 12px;
      padding: 14px;
    }
    .mark {
      background: var(--agora-evidence-accent);
      border-radius: var(--agora-radius-control);
      height: 28px;
      width: 28px;
    }
    .title {
      font-weight: 700;
      min-width: 120px;
    }
    .search {
      background: var(--agora-surface-raised);
      border: 1px solid var(--agora-border);
      border-radius: var(--agora-radius-control);
      color: var(--agora-fg);
      flex: 1 1 auto;
      font: inherit;
      height: var(--agora-control-height);
      min-width: 180px;
      padding: 0 12px;
    }
    .close {
      background: var(--agora-surface-raised);
      border: 1px solid var(--agora-border);
      border-radius: var(--agora-radius-control);
      color: var(--agora-fg);
      font: inherit;
      height: var(--agora-control-height);
      min-width: 72px;
    }
    .launcher-body {
      display: grid;
      grid-template-columns: 176px minmax(0, 1fr);
      min-height: 0;
      overflow: hidden;
    }
    .categories {
      background: var(--agora-evidence-strong);
      border-right: 1px solid var(--agora-border-subtle);
      display: flex;
      flex-direction: column;
      gap: 6px;
      overflow-y: auto;
      padding: 12px;
    }
    .category {
      background: transparent;
      border: 1px solid transparent;
      border-radius: var(--agora-radius-control);
      color: var(--agora-fg);
      font: inherit;
      min-height: 36px;
      padding: 0 10px;
      text-align: left;
    }
    .category.active {
      background: var(--agora-surface-raised);
      border-color: var(--agora-accent);
    }
    .apps {
      display: grid;
      grid-template-rows: auto 1fr;
      min-height: 0;
      min-width: 0;
      overflow: hidden;
    }
    .summary {
      border-bottom: 1px solid var(--agora-border-subtle);
      color: var(--agora-text-muted);
      font-size: 13px;
      padding: 10px 14px;
    }
    .app-list {
      display: flex;
      flex-direction: column;
      gap: 8px;
      min-height: 0;
      overflow-y: auto;
      padding: 12px;
    }
    .app {
      align-items: center;
      background: var(--agora-surface-raised);
      border: 1px solid var(--agora-border-subtle);
      border-radius: var(--agora-radius-control);
      color: var(--agora-fg);
      display: grid;
      gap: 10px;
      grid-template-columns: 34px minmax(0, 1fr);
      min-height: 54px;
      padding: 8px 10px;
      text-align: left;
    }
    .app:disabled {
      opacity: 0.72;
    }
    .app:not(:disabled) {
      cursor: pointer;
    }
    .app:not(:disabled):hover,
    .app:not(:disabled):focus-visible {
      border-color: var(--agora-accent);
    }
    .app-icon {
      align-items: center;
      background: var(--agora-evidence-strong);
      border-radius: var(--agora-radius-control);
      color: var(--agora-fg);
      display: inline-flex;
      font-size: 14px;
      height: 34px;
      justify-content: center;
      width: 34px;
    }
    .app-name,
    .app-detail {
      display: block;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
    .app-detail {
      color: var(--agora-text-muted);
      font-size: 12px;
      margin-top: 3px;
    }
    .footer {
      align-items: center;
      border-top: 1px solid var(--agora-border-subtle);
      color: var(--agora-text-muted);
      display: flex;
      font-size: 13px;
      justify-content: space-between;
      min-height: 42px;
      padding: 0 14px;
    }
  </style>
</head>
<body data-surface="launcher">
  <main class="launcher" aria-label="Agora DE app launcher">
    <header class="launcher-header">
      <span class="mark"></span>
      <span class="title">Applications</span>
      <input class="search" id="app-search" type="search" aria-label="Search apps" placeholder="Search">
      <button class="close" id="close-button" type="button">Close</button>
    </header>
    <section class="launcher-body">
      <nav class="categories" id="categories" aria-label="Application categories"></nav>
      <section class="apps" aria-label="Applications">
        <div class="summary" id="summary">loading apps</div>
        <div class="app-list" id="app-list"></div>
      </section>
    </section>
    <footer class="footer">
      <span id="status">loading</span>
      <span id="policy-status">checking apps</span>
    </footer>
  </main>
  <script>
    const state = {
      apps: [],
      query: "",
      category: "All"
    };

    function text(value, fallback) {
      const trimmed = String(value || "").trim();
      return trimmed || fallback;
    }

    function categories() {
      const names = new Set(["All"]);
      state.apps.forEach((app) => names.add(text(app.category, "Other")));
      return Array.from(names).sort((left, right) => left === "All" ? -1 : right === "All" ? 1 : left.localeCompare(right));
    }

    function filteredApps() {
      const query = state.query.trim().toLowerCase();
      return state.apps.filter((app) => {
        const category = text(app.category, "Other");
        if (state.category !== "All" && category !== state.category) {
          return false;
        }
        if (!query) {
          return true;
        }
        const haystack = [
          text(app.name, app.id),
          text(app.id, ""),
          category,
          ...(Array.isArray(app.categories) ? app.categories : [])
        ].join(" ").toLowerCase();
        return haystack.includes(query);
      });
    }

    function renderCategories() {
      const target = document.getElementById("categories");
      target.replaceChildren();
      categories().forEach((category) => {
        const button = document.createElement("button");
        button.type = "button";
        button.className = "category" + (category === state.category ? " active" : "");
        button.textContent = category;
        button.addEventListener("click", () => {
          state.category = category;
          render();
        });
        target.appendChild(button);
      });
    }

    function renderApp(app) {
      const label = text(app.name, app.id);
      const reason = text(app.disabledReason, app.launchable ? "" : "not launchable");
      const button = document.createElement("button");
      button.type = "button";
      button.className = "app";
      button.disabled = !app.launchable;
      button.dataset.appId = app.id;
      button.dataset.disabledCode = text(app.disabledCode, "");
      button.setAttribute("aria-disabled", String(!app.launchable));
      button.title = reason ? label + " - " + reason : label;
      button.addEventListener("click", () => launchApp(app.id));

      const icon = document.createElement("span");
      icon.className = "app-icon";
      icon.textContent = text(app.iconLabel, label.slice(0, 1).toUpperCase());
      icon.title = text(app.iconRef, text(app.icon, ""));
      const copy = document.createElement("span");
      const name = document.createElement("span");
      name.className = "app-name";
      name.textContent = label;
      const detail = document.createElement("span");
      detail.className = "app-detail";
      detail.textContent = reason ? reason : text(app.category, "Other");
      copy.appendChild(name);
      copy.appendChild(detail);
      button.appendChild(icon);
      button.appendChild(copy);
      return button;
    }

    function render() {
      renderCategories();
      const apps = filteredApps();
      const list = document.getElementById("app-list");
      list.replaceChildren();
      if (!apps.length) {
        const empty = document.createElement("div");
        empty.className = "app-detail";
        empty.textContent = "no matching apps";
        list.appendChild(empty);
      } else {
        apps.forEach((app) => list.appendChild(renderApp(app)));
      }
      document.getElementById("summary").textContent = apps.length + " of " + state.apps.length + " apps";
      document.getElementById("status").textContent = state.category + (state.query ? " search" : "");
      const launchable = state.apps.filter((app) => app.launchable === true).length;
      const disabled = state.apps.length - launchable;
      document.getElementById("policy-status").textContent = launchable + " launchable / " + disabled + " disabled";
    }

    async function loadJSON(path) {
      const response = await fetch(path, {cache: "no-store"});
      if (!response.ok) {
        throw new Error(path + " returned " + response.status);
      }
      return response.json();
    }

    async function postJSON(path, body) {
      const response = await fetch(path, {
        method: "POST",
        headers: {"Content-Type": "application/json"},
        body: JSON.stringify(body)
      });
      if (!response.ok) {
        let errorBody = {};
        try {
          errorBody = await response.json();
        } catch (_error) {
          errorBody = {};
        }
        const error = new Error(text(errorBody.error, path + " returned " + response.status));
        error.status = response.status;
        error.errorClass = text(errorBody.errorClass, "");
        throw error;
      }
      return response.json();
    }

    async function refresh() {
      try {
        const catalog = await loadJSON("/api/catalog/apps");
        state.apps = Array.isArray(catalog.apps) ? catalog.apps : [];
        render();
      } catch (error) {
        document.getElementById("summary").textContent = "catalog offline";
        document.getElementById("status").textContent = "offline";
      }
    }

    async function launcherSurface() {
      const surfaces = await loadJSON("/api/surfaces");
      return (Array.isArray(surfaces.surfaces) ? surfaces.surfaces : []).find((surface) =>
        surface.mapped && surface.appId === "io.agorade.ShellLauncher"
      );
    }

    async function closeLauncher() {
      try {
        const surface = await launcherSurface();
        if (surface) {
          await postJSON("/api/surfaces/action", {surfaceId: surface.id, action: "close"});
        } else {
          window.close();
        }
      } catch (error) {
        window.close();
      }
    }

    async function launchApp(appId) {
      document.getElementById("status").textContent = "launching";
      try {
        await postJSON("/api/catalog/launch", {appId});
        document.getElementById("status").textContent = "launch accepted";
        await closeLauncher();
      } catch (error) {
        document.getElementById("status").textContent = "launch failed";
      }
    }

    document.getElementById("app-search").addEventListener("input", (event) => {
      state.query = event.target.value;
      render();
    });
    document.getElementById("close-button").addEventListener("click", closeLauncher);
    document.addEventListener("keydown", (event) => {
      if (event.key === "Escape") {
        closeLauncher();
      }
    });
    refresh().then(() => document.getElementById("app-search").focus());
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
      background: transparent !important;
      color: var(--agora-fg);
      height: 100%%;
      margin: 0;
      overflow: hidden;
      width: 100%%;
    }
    body {
      align-items: stretch;
      background: transparent;
      box-sizing: border-box;
      display: flex;
      font: var(--agora-font-panel);
      overflow: hidden;
    }
    .panel {
      align-items: center;
      background: var(--agora-surface);
      border-top: 4px solid var(--agora-evidence-accent);
      bottom: 0;
      box-shadow: inset 0 1px 0 var(--agora-border-subtle);
      box-sizing: border-box;
      display: flex;
      gap: var(--agora-panel-gap);
      height: var(--agora-panel-height);
      left: 0;
      min-height: var(--agora-panel-height);
      padding: 0 var(--agora-panel-padding-x);
      position: fixed;
      right: 0;
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
    .layout-status,
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
    .layout-status,
    .status,
    .clock {
      border: 2px solid var(--agora-border);
      color: var(--agora-fg);
    }
    button.workspace {
      background: var(--agora-surface-raised);
      font: inherit;
    }
    .layout-status {
      background: var(--agora-surface-raised);
      font: inherit;
      min-width: 88px;
    }
    .dock-section {
      align-items: center;
      display: flex;
      gap: 10px;
      min-width: 0;
    }
    .running {
      flex: 1 1 auto;
      overflow: hidden;
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
    .wm-controls {
      flex: 0 1 auto;
      max-width: 760px;
      overflow-x: auto;
      padding-bottom: 2px;
    }
    .wm-control {
      background: var(--agora-surface);
      border: 2px solid var(--agora-border);
      border-radius: var(--agora-radius-control);
      color: var(--agora-fg);
      flex: 0 0 auto;
      height: var(--agora-control-height);
      min-width: 54px;
      padding: 0 10px;
    }
    .wm-control.primary {
      border-color: var(--agora-accent);
    }
    .wm-control:disabled {
      color: var(--agora-text-muted);
      cursor: default;
      opacity: 0.65;
    }
    .wm-rule {
      background: var(--agora-surface-strong);
      border-color: var(--agora-border-subtle);
      max-width: 150px;
      min-width: 96px;
    }
    .surface-meta {
      font-size: 12px;
      min-width: 78px;
      padding: 0 8px;
    }
    .target-meta {
      border-color: var(--agora-accent);
      max-width: 170px;
      min-width: 104px;
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
    <section class="dock-section running" id="running-list" aria-label="Running surfaces">
      <span class="dock-item muted">loading surfaces</span>
    </section>
    <section class="dock-section wm-controls" id="wm-controls" aria-label="Window controls">
      <span class="dock-item surface-meta target-meta" id="target-label">no target</span>
      <button class="wm-control" id="focus-prev-button" type="button">Prev</button>
      <button class="wm-control" id="focus-next-button" type="button">Next</button>
      <button class="wm-control primary" id="promote-button" type="button">Master</button>
      <button class="wm-control" id="move-zone-button" type="button">Move</button>
      <button class="wm-control" id="float-button" type="button">Float</button>
      <button class="wm-control" id="fullscreen-button" type="button">Full</button>
      <button class="wm-control" id="maximize-button" type="button">Max</button>
      <button class="wm-control" id="close-focus-button" type="button">Close</button>
      <button class="wm-control" id="reset-layout-button" type="button">Reset</button>
      <button class="wm-control" id="rule-button" type="button">Rule</button>
      <button class="wm-control" id="master-count-button" type="button">M1</button>
      <button class="wm-control" id="master-ratio-button" type="button">50%%</button>
      <button class="wm-control" id="gaps-button" type="button">Gap0</button>
      <button class="wm-control" id="smart-gaps-button" type="button">Smart</button>
      <span class="dock-item surface-meta wm-rule" id="layout-rule-label">master_stack</span>
    </section>
    <button class="workspace" id="workspace-label" type="button">workspace 1</button>
    <button class="layout-status" id="layout-mode-button" type="button">freeform</button>
    <span class="status" id="status-label">starting</span>
    <time class="clock" id="clock-label">--:--</time>
  </main>
  <script>
    const state = {
      apps: [],
      surfaces: [],
      layout: {mode: "freeform", revision: 0, surfaces: [], workspaces: []},
      workspace: {id: "workspace-1", name: "workspace 1", active: true, surfaceCount: 0},
      feedback: {label: "", className: "", until: 0},
      surface: %q
    };
    const unsupportedSurfaceActions = new Set(["fullscreen", "maximize"]);

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

    function launcherSurface() {
      return state.surfaces.find((surface) =>
        surface.mapped && surface.appId === "io.agorade.ShellLauncher"
      );
    }

    function layoutSurface(surfaceId) {
      const surfaces = Array.isArray(state.layout.surfaces) ? state.layout.surfaces : [];
      return surfaces.find((surface) => surface.surfaceId === surfaceId);
    }

    function layoutSurfaces() {
      return Array.isArray(state.layout.surfaces) ? state.layout.surfaces : [];
    }

    function manageableLayoutSurfaces() {
      return layoutSurfaces()
        .filter((surface) => surface.visible !== false && surface.participation !== "transient")
        .sort((left, right) => Number(left.order || 0) - Number(right.order || 0));
    }

    function focusedLayoutSurface() {
      return manageableLayoutSurfaces().find((surface) => surface.focused) || manageableLayoutSurfaces()[0] || null;
    }

    function targetSurface() {
      const layout = focusedLayoutSurface();
      if (layout) {
        return layout;
      }
      const surface = state.surfaces.find((candidate) =>
        candidate.mapped &&
        candidate.surfaceKind !== "layer_shell" &&
        candidate.appId !== "io.agorade.ShellLauncher" &&
        candidate.focused
      ) || state.surfaces.find((candidate) =>
        candidate.mapped &&
        candidate.surfaceKind !== "layer_shell" &&
        candidate.appId !== "io.agorade.ShellLauncher"
      );
      return surface ? {surfaceId: surface.id, appId: surface.appId, title: surface.title, zoneId: surface.zoneId, focused: surface.focused, floating: surface.layoutRole === "floating"} : null;
    }

    function activeLayoutWorkspace() {
      const workspaces = Array.isArray(state.layout.workspaces) ? state.layout.workspaces : [];
      return workspaces.find((workspace) => workspace.active) ||
        workspaces.find((workspace) => workspace.id === state.workspace.id) ||
        workspaces[0] ||
        {zones: []};
    }

    function workspaceZones() {
      const zones = Array.isArray(activeLayoutWorkspace().zones) ? activeLayoutWorkspace().zones : [];
      const zoneIds = zones
        .filter((zone) => text(zone.id, "") && zone.kind !== "chrome")
        .map((zone) => zone.id);
      return zoneIds.length ? zoneIds : ["primary", "secondary"];
    }

    function nextLayoutMode(mode) {
      const modes = ["freeform", "zones", "columns"];
      const current = modes.indexOf(mode);
      return modes[(current + 1) %% modes.length];
    }

    function nextZone(zoneId) {
      const zones = workspaceZones();
      const index = zones.indexOf(zoneId);
      return zones[(index + 1) %% zones.length];
    }

    function geometryLabel(geometry) {
      if (!geometry || geometry.width <= 0 || geometry.height <= 0) {
        return "";
      }
      return geometry.width + "x" + geometry.height + "@" + geometry.x + "," + geometry.y;
    }

    function surfaceAreaLabel(layout, zone) {
      const area = geometryLabel(layout.geometry);
      return area ? zone + " " + area : zone;
    }

    function layoutSettingsLabel() {
      const settings = state.layout.settings || {};
      const gaps = settings.gaps || {};
      const gapLabel = [gaps.outerHorizontal, gaps.outerVertical, gaps.innerHorizontal, gaps.innerVertical]
        .filter((value) => Number(value || 0) > 0)
        .length ? " gaps" : "";
      const master = settings.masterCount ? " n" + settings.masterCount : "";
      const ratio = settings.masterRatio ? " " + Math.round(settings.masterRatio * 100) + "%%" : "";
      return text(settings.rule, "master_stack") + master + ratio + gapLabel;
    }

    function normalizedSettings() {
      const settings = state.layout.settings || {};
      const gaps = settings.gaps || {};
      return {
        rule: text(settings.rule, "master_stack"),
        mode: text(settings.mode, text(state.layout.mode, "zones")),
        gaps: {
          outerHorizontal: Math.max(0, Number(gaps.outerHorizontal || 0)),
          outerVertical: Math.max(0, Number(gaps.outerVertical || 0)),
          innerHorizontal: Math.max(0, Number(gaps.innerHorizontal || 0)),
          innerVertical: Math.max(0, Number(gaps.innerVertical || 0))
        },
        masterCount: Math.max(1, Number(settings.masterCount || 1)),
        masterRatio: Math.min(0.9, Math.max(0.1, Number(settings.masterRatio || 0.5))),
        smartGaps: settings.smartGaps !== false
      };
    }

    function nextLayoutRule(rule) {
      const rules = ["master_stack", "zones", "dwindle"];
      const current = rules.indexOf(rule);
      return rules[(current + 1) %% rules.length];
    }

    function nextMasterCount(count) {
      return count >= 3 ? 1 : count + 1;
    }

    function nextMasterRatio(ratio) {
      const rounded = Math.round(ratio * 10) / 10;
      return rounded >= 0.8 ? 0.4 : Math.round((rounded + 0.1) * 10) / 10;
    }

    function nextGapSet(gaps) {
      const values = [gaps.outerHorizontal, gaps.outerVertical, gaps.innerHorizontal, gaps.innerVertical];
      const current = Math.max.apply(null, values.map((value) => Number(value || 0)));
      const next = current >= 16 ? 0 : current + 4;
      return {
        outerHorizontal: next,
        outerVertical: next,
        innerHorizontal: next,
        innerVertical: next
      };
    }

    function setFeedback(label, className) {
      state.feedback = {
        label,
        className: className || "ready",
        until: Date.now() + 3500
      };
    }

    function statusFromFeedback() {
      if (state.feedback.label && Date.now() < state.feedback.until) {
        return state.feedback;
      }
      return null;
    }

    function actionStatus(result) {
      return text(result && (result.status || result.decision), "accepted");
    }

    function setControlDisabled(id, disabled) {
      const element = document.getElementById(id);
      if (element) {
        element.disabled = disabled;
        element.setAttribute("aria-disabled", String(disabled));
      }
    }

    function renderWMControls() {
      const target = targetSurface();
      const hasTarget = Boolean(target && target.surfaceId);
      [
        "focus-prev-button",
        "focus-next-button",
        "promote-button",
        "move-zone-button",
        "float-button",
        "fullscreen-button",
        "maximize-button",
        "close-focus-button"
      ].forEach((id) => setControlDisabled(id, !hasTarget));
      [
        ["fullscreen-button", "Fullscreen is unsupported by the current compositor backend"],
        ["maximize-button", "Maximize is unsupported by the current compositor backend"]
      ].forEach(([id, title]) => {
        const element = document.getElementById(id);
        if (element) {
          element.disabled = true;
          element.setAttribute("aria-disabled", "true");
          element.title = title;
        }
      });
      const targetLabel = document.getElementById("target-label");
      if (targetLabel) {
        const targetName = target ? text(target.title, text(target.appId, target.surfaceId)) : "no target";
        const targetZone = target ? text(target.zoneId, "primary") : "";
        targetLabel.textContent = target ? targetName : "no target";
        targetLabel.title = target ? targetName + " / " + targetZone + " / " + geometryLabel(target.geometry) : "no target";
      }
      const floatButton = document.getElementById("float-button");
      if (floatButton) {
        floatButton.textContent = target && target.floating ? "Tile" : "Float";
        floatButton.title = target ? "Toggle floating for " + text(target.title, text(target.appId, target.surfaceId)) : "Toggle floating";
      }
      const moveButton = document.getElementById("move-zone-button");
      if (moveButton && target) {
        moveButton.title = "Move " + text(target.title, target.surfaceId) + " to " + nextZone(text(target.zoneId, "primary"));
      }
      const ruleLabel = document.getElementById("layout-rule-label");
      const settings = state.layout.settings || {};
      ruleLabel.textContent = layoutSettingsLabel();
      ruleLabel.title = "rule " + text(settings.rule, "master_stack") +
        " / mode " + text(state.layout.mode, "freeform") +
        " / revision " + Number(state.layout.revision || 0);
      const normalized = normalizedSettings();
      const ruleButton = document.getElementById("rule-button");
      ruleButton.textContent = text(normalized.rule, "master_stack").replace("_stack", "");
      ruleButton.title = "rule " + normalized.rule;
      const masterCountButton = document.getElementById("master-count-button");
      masterCountButton.textContent = "M" + normalized.masterCount;
      masterCountButton.title = "master count " + normalized.masterCount;
      const masterRatioButton = document.getElementById("master-ratio-button");
      masterRatioButton.textContent = Math.round(normalized.masterRatio * 100) + "%%";
      masterRatioButton.title = "master ratio " + normalized.masterRatio.toFixed(2);
      const gapsButton = document.getElementById("gaps-button");
      const gapValue = Math.max(normalized.gaps.outerHorizontal, normalized.gaps.outerVertical, normalized.gaps.innerHorizontal, normalized.gaps.innerVertical);
      gapsButton.textContent = "Gap" + gapValue;
      gapsButton.title = "gaps " + normalized.gaps.outerHorizontal + "/" + normalized.gaps.outerVertical + "/" + normalized.gaps.innerHorizontal + "/" + normalized.gaps.innerVertical;
      const smartGapsButton = document.getElementById("smart-gaps-button");
      smartGapsButton.textContent = normalized.smartGaps ? "Smart" : "Gaps";
      smartGapsButton.title = normalized.smartGaps ? "smart gaps on" : "smart gaps off";
    }

    function render() {
      const launcher = launcherSurface();
      const showingApps = Boolean(launcher);
      document.querySelector(".panel").classList.toggle("apps-open", showingApps);
      const appsButton = document.getElementById("apps-button");
      appsButton.textContent = showingApps ? "Hide Apps" : "Apps";
      appsButton.title = showingApps ? "Close applications" : state.apps.length + " apps";
      appsButton.setAttribute("aria-pressed", showingApps ? "true" : "false");
      const workSurfaces = state.surfaces.filter((surface) =>
        surface.mapped &&
        surface.surfaceKind !== "layer_shell" &&
        surface.appId !== "io.agorade.ShellLauncher"
      );
      renderList("running-list", "no running apps", workSurfaces, (surface) => {
        const group = document.createElement("span");
        group.className = "surface-actions";
        const layout = layoutSurface(surface.id) || {};
        const label = text(layout.label, text(surface.title, text(surface.appId, surface.id)));
        const zone = text(layout.zoneId, text(surface.zoneId, "primary"));
        const focusButton = button(label, "dock-item" + (surface.focused || layout.focused ? " focused" : ""), () => actOnSurface(surface.id, "focus"));
        const area = surfaceAreaLabel(layout, zone);
        focusButton.title = text(surface.title, text(surface.appId, surface.id)) + " / " + area;
        group.appendChild(focusButton);
        group.appendChild(item(area, "surface-meta muted"));
        group.appendChild(button("Zone", "surface-action", () => assignZone(surface.id, nextZone(zone))));
        group.appendChild(button("Close", "surface-action", () => actOnSurface(surface.id, "close")));
        return group;
      });
      const status = document.getElementById("status-label");
      const feedback = statusFromFeedback();
      if (feedback) {
        status.textContent = feedback.label;
        status.className = "status " + feedback.className;
      } else if (launcher) {
        status.textContent = "apps open";
        status.className = "status ready";
      } else {
        status.textContent = workSurfaces.length ? workSurfaces.length + " running" : "ready";
        status.className = "status " + (workSurfaces.length ? "ready" : "warn");
      }
      const workspace = document.getElementById("workspace-label");
      workspace.textContent = text(state.workspace.name, "workspace 1");
      workspace.title = text(state.layout.mode, "freeform") + (state.workspace.surfaceCount ? " / " + state.workspace.surfaceCount + " work surfaces" : "");
      const layoutMode = text(state.layout.mode, "freeform");
      const layoutStatus = document.getElementById("layout-mode-button");
      layoutStatus.textContent = layoutMode + (state.layout.revision ? " r" + state.layout.revision : "");
      layoutStatus.title = layoutSettingsLabel() + " / zones: " + workspaceZones().join(" / ");
      renderWMControls();
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
        const [catalog, surfaces, workspaces, layout] = await Promise.all([
          loadJSON("/api/catalog/apps"),
          loadJSON("/api/surfaces"),
          loadJSON("/api/workspaces"),
          loadJSON("/api/layout")
        ]);
        state.apps = Array.isArray(catalog.apps) ? catalog.apps : [];
        state.surfaces = Array.isArray(surfaces.surfaces) ? surfaces.surfaces : [];
        state.layout = layout.layout || state.layout;
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
        const result = await postJSON("/api/catalog/launch", {appId});
        await refresh();
        setFeedback(actionStatus(result), "ready");
        render();
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
        const result = await postJSON("/api/surfaces/action", {surfaceId, action});
        await refresh();
        setFeedback(action + " " + actionStatus(result), "ready");
        render();
      } catch (error) {
        status.textContent = error.errorClass === "backend_unsupported" ? action + " unsupported" : action + " failed";
        status.className = "status warn";
      }
    }

    async function assignZone(surfaceId, zoneId) {
      const status = document.getElementById("status-label");
      status.textContent = "zone";
      status.className = "status ready";
      try {
        const target = layoutSurface(surfaceId) || {};
        const result = await postJSON("/api/layout/action", {
          surfaceId,
          workspaceId: text(target.workspaceId, state.workspace.id),
          zoneId,
          action: "assignZone"
        });
        await refresh();
        setFeedback("zone " + actionStatus(result), "ready");
        render();
      } catch (error) {
        status.textContent = "zone unsupported";
        status.className = "status warn";
      }
    }

    async function setLayoutMode(mode) {
      const status = document.getElementById("status-label");
      status.textContent = mode;
      status.className = "status ready";
      try {
        const result = await postJSON("/api/layout/action", {mode, action: "setMode"});
        await refresh();
        setFeedback("layout " + actionStatus(result), "ready");
        render();
      } catch (error) {
        status.textContent = "layout unsupported";
        status.className = "status warn";
      }
    }

    async function setLayoutSettings(patch) {
      const status = document.getElementById("status-label");
      const current = normalizedSettings();
      const settings = Object.assign({}, current, patch || {});
      settings.gaps = Object.assign({}, current.gaps, (patch && patch.gaps) || {});
      status.textContent = "settings";
      status.className = "status ready";
      try {
        const result = await postJSON("/api/layout/action", {action: "setSettings", settings});
        await refresh();
        setFeedback("settings " + actionStatus(result), "ready");
        render();
      } catch (error) {
        status.textContent = "settings unsupported";
        status.className = "status warn";
      }
    }

    async function cycleLayoutRule() {
      const settings = normalizedSettings();
      await setLayoutSettings({rule: nextLayoutRule(settings.rule)});
    }

    async function cycleMasterCount() {
      const settings = normalizedSettings();
      await setLayoutSettings({masterCount: nextMasterCount(settings.masterCount)});
    }

    async function cycleMasterRatio() {
      const settings = normalizedSettings();
      await setLayoutSettings({masterRatio: nextMasterRatio(settings.masterRatio)});
    }

    async function cycleGaps() {
      const settings = normalizedSettings();
      await setLayoutSettings({gaps: nextGapSet(settings.gaps)});
    }

    async function toggleSmartGaps() {
      const settings = normalizedSettings();
      await setLayoutSettings({smartGaps: !settings.smartGaps});
    }

    async function activateWorkspace() {
      const status = document.getElementById("status-label");
      status.textContent = "workspace";
      status.className = "status ready";
      try {
        if (state.layout.mode !== "zones") {
          await setLayoutMode("zones");
        } else {
          await postJSON("/api/workspaces/action", {workspaceId: "workspace-1", action: "activate"});
        }
        await refresh();
      } catch (error) {
        status.textContent = "workspace failed";
        status.className = "status warn";
      }
    }

    async function focusRelative(delta) {
      const surfaces = manageableLayoutSurfaces();
      const target = targetSurface();
      if (!surfaces.length || !target) {
        return;
      }
      const current = surfaces.findIndex((surface) => surface.surfaceId === target.surfaceId);
      const next = surfaces[(Math.max(current, 0) + delta + surfaces.length) %% surfaces.length];
      if (next) {
        await promoteSurface(next.surfaceId);
      }
    }

    async function promoteSurface(surfaceId) {
      const status = document.getElementById("status-label");
      status.textContent = "promote";
      status.className = "status ready";
      try {
        const result = await postJSON("/api/layout/action", {surfaceId, action: "promote"});
        await refresh();
        setFeedback("promote " + actionStatus(result), "ready");
        render();
      } catch (error) {
        status.textContent = "promote failed";
        status.className = "status warn";
      }
    }

    async function promoteTarget() {
      const target = targetSurface();
      if (!target) {
        return;
      }
      await promoteSurface(target.surfaceId);
    }

    async function moveTargetToNextZone() {
      const target = targetSurface();
      if (!target) {
        return;
      }
      await assignZone(target.surfaceId, nextZone(text(target.zoneId, "primary")));
    }

    async function toggleTargetFloating() {
      const target = targetSurface();
      if (!target) {
        return;
      }
      const floating = !Boolean(target.floating);
      const status = document.getElementById("status-label");
      status.textContent = floating ? "floating" : "tiling";
      status.className = "status ready";
      try {
        const result = await postJSON("/api/layout/action", {surfaceId: target.surfaceId, floating, action: "setFloating"});
        await refresh();
        setFeedback((floating ? "float " : "tile ") + actionStatus(result), "ready");
        render();
      } catch (error) {
        status.textContent = "float failed";
        status.className = "status warn";
      }
    }

    async function actOnTarget(action) {
      if (unsupportedSurfaceActions.has(action)) {
        const status = document.getElementById("status-label");
        status.textContent = action + " unsupported";
        status.className = "status warn";
        return;
      }
      const target = targetSurface();
      if (target) {
        await actOnSurface(target.surfaceId, action);
      }
    }

    async function resetLayout() {
      const mode = text(state.layout.mode, "freeform");
      await setLayoutMode(mode === "freeform" ? "zones" : mode);
    }

    async function toggleApps() {
      const status = document.getElementById("status-label");
      const launcher = launcherSurface();
      if (launcher) {
        status.textContent = "apps";
        status.className = "status ready";
        try {
          await postJSON("/api/surfaces/action", {surfaceId: launcher.id, action: "close"});
          await refresh();
        } catch (error) {
          status.textContent = "close failed";
          status.className = "status warn";
        }
        return;
      }
      await launchApp("shell-launcher");
    }

    function updateClock() {
      const now = new Date();
      document.getElementById("clock-label").textContent = now.toLocaleTimeString([], {
        hour: "2-digit",
        minute: "2-digit"
      });
    }

    document.getElementById("apps-button").addEventListener("click", toggleApps);
    document.getElementById("refresh-button").addEventListener("click", refresh);
    document.getElementById("operator-button").addEventListener("click", () => launchApp("shell-status"));
    document.getElementById("workspace-label").addEventListener("click", activateWorkspace);
    document.getElementById("layout-mode-button").addEventListener("click", () => setLayoutMode(nextLayoutMode(text(state.layout.mode, "freeform"))));
    document.getElementById("focus-prev-button").addEventListener("click", () => focusRelative(-1));
    document.getElementById("focus-next-button").addEventListener("click", () => focusRelative(1));
    document.getElementById("promote-button").addEventListener("click", promoteTarget);
    document.getElementById("move-zone-button").addEventListener("click", moveTargetToNextZone);
    document.getElementById("float-button").addEventListener("click", toggleTargetFloating);
    document.getElementById("fullscreen-button").addEventListener("click", () => actOnTarget("fullscreen"));
    document.getElementById("maximize-button").addEventListener("click", () => actOnTarget("maximize"));
    document.getElementById("close-focus-button").addEventListener("click", () => actOnTarget("close"));
    document.getElementById("reset-layout-button").addEventListener("click", resetLayout);
    document.getElementById("rule-button").addEventListener("click", cycleLayoutRule);
    document.getElementById("master-count-button").addEventListener("click", cycleMasterCount);
    document.getElementById("master-ratio-button").addEventListener("click", cycleMasterRatio);
    document.getElementById("gaps-button").addEventListener("click", cycleGaps);
    document.getElementById("smart-gaps-button").addEventListener("click", toggleSmartGaps);
    updateClock();
    refresh();
    setInterval(updateClock, 30000);
    setInterval(refresh, 3000);
  </script>
</body>
</html>`, theme.MustDefaultTokenCSS(), escapedSurface, escapedSurface)
}
