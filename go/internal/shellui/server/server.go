package server

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
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

//go:embed shellassets/*.html
var embeddedShellAssets embed.FS

const (
	DefaultListenAddress  = "127.0.0.1:7780"
	LayoutPath            = layoutroute.LayoutPath
	LayoutActionPath      = layoutroute.ActionPath
	WorkControlsPath      = "/api/work-surface-controls"
	SurfaceActionPath     = "/api/surfaces/action"
	OperatorStatusPath    = "/api/operator/status"
	TimingDiagnosticsPath = "/api/diagnostics/timing"
	ThemePath             = "/api/theme"
	SettingsPath          = "/api/settings"
	WorkspacesPath        = "/api/workspaces"
	WorkspaceActionPath   = "/api/workspaces/action"
	CatalogIconPathPrefix = "/api/catalog/icons/"

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
	ThemeID                  string
	ThemeManifestPath        string
	CatalogProvider          string
	DesktopEntryRoots        []string
	IconThemeRoots           []string
	IconPixmapRoots          []string
	SurfaceProvider          string
	CompositorctlPath        string
	SystemctlPath            string
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
	catalogProvider, launchProvider, surfaceProvider, iconFiles, err := providers(config)
	if err != nil {
		return nil, err
	}
	themeSelection := theme.Resolve(theme.SelectionOptions{
		ID:           config.ThemeID,
		ManifestPath: config.ThemeManifestPath,
	})

	useCompositorctl := strings.TrimSpace(config.SurfaceProvider) == SurfaceProviderCompositorctl
	timings := newTimingRecorder(timingConfig{UseCompositorctl: useCompositorctl})
	mux := http.NewServeMux()
	mux.Handle(catalogroute.AppsPath, catalogroute.New(catalogProvider, launchProvider))
	mux.Handle(catalogroute.LaunchPath, catalogroute.New(catalogProvider, launchProvider))
	mux.Handle(CatalogIconPathPrefix, catalogIconHandler(iconFiles))
	mux.Handle(surfaceroute.SurfacesPath, surfaceroute.New(surfaceProvider))
	mux.Handle(LayoutPath, layoutroute.New(layoutroute.Config{
		CompositorctlPath: config.CompositorctlPath,
		UseCompositorctl:  useCompositorctl,
		SurfaceProvider:   surfaceProvider,
	}))
	mux.Handle(LayoutActionPath, layoutroute.NewAction(layoutroute.Config{CompositorctlPath: config.CompositorctlPath}))
	mux.Handle(WorkControlsPath, surfaceroute.Handler{
		Path:     WorkControlsPath,
		Provider: surfaceProvider,
	})
	mux.Handle(SurfaceActionPath, surfaceActionHandler(config))
	mux.Handle(OperatorStatusPath, operatorStatusHandler(config, surfaceProvider, timings))
	mux.Handle(TimingDiagnosticsPath, timingDiagnosticsHandler(timings))
	mux.Handle(ThemePath, themeHandler(themeSelection))
	mux.Handle(SettingsPath, settingsHandler(config))
	workspaceConfig := workspaceRouteConfig{
		CompositorctlPath: config.CompositorctlPath,
		UseCompositorctl:  useCompositorctl,
		SurfaceProvider:   surfaceProvider,
	}
	mux.Handle(WorkspacesPath, workspacesHandler(workspaceConfig))
	mux.Handle(WorkspaceActionPath, workspaceActionHandler(workspaceConfig))
	mux.Handle("/shell/dist/", shellAssetHandler(config.StaticRoot, themeSelection.CSS))
	return noStore(timings.instrument(mux)), nil
}

type themeResponse struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Source         string `json:"source"`
	Fallback       bool   `json:"fallback"`
	FallbackReason string `json:"fallbackReason,omitempty"`
}

func themeHandler(selection theme.Selection) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			response.Header().Set("Allow", http.MethodGet)
			writeJSON(response, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		writeJSON(response, http.StatusOK, themeResponse{
			ID:             selection.Manifest.ID,
			Name:           selection.Manifest.Name,
			Source:         selection.Source,
			Fallback:       selection.FallbackReason != "",
			FallbackReason: selection.FallbackReason,
		})
	})
}

type settingsResponse struct {
	GeneratedAtUnixMillis    int64               `json:"generatedAtUnixMillis"`
	DiagnosticOverlayEnabled bool                `json:"diagnosticOverlayEnabled"`
	DiagnosticOverlay        shellServiceSetting `json:"diagnosticOverlay"`
}

type shellServiceSetting struct {
	Name         string `json:"name"`
	Scope        string `json:"scope"`
	EnabledState string `json:"enabledState"`
	ActiveState  string `json:"activeState"`
	Enabled      bool   `json:"enabled"`
	Active       bool   `json:"active"`
}

type settingsUpdateRequest struct {
	DiagnosticOverlayEnabled *bool `json:"diagnosticOverlayEnabled,omitempty"`
}

func settingsHandler(config Config) http.Handler {
	systemctl := strings.TrimSpace(config.SystemctlPath)
	if systemctl == "" {
		systemctl = "systemctl"
	}
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != SettingsPath {
			http.NotFound(response, request)
			return
		}
		ctx, cancel := context.WithTimeout(request.Context(), 5*time.Second)
		defer cancel()
		switch request.Method {
		case http.MethodGet:
			writeJSON(response, http.StatusOK, collectSettings(ctx, systemctl, time.Now()))
		case http.MethodPost:
			var update settingsUpdateRequest
			if err := json.NewDecoder(request.Body).Decode(&update); err != nil {
				writeJSON(response, http.StatusBadRequest, map[string]string{"error": "invalid settings request"})
				return
			}
			if update.DiagnosticOverlayEnabled == nil {
				writeJSON(response, http.StatusBadRequest, map[string]string{"error": "diagnosticOverlayEnabled is required"})
				return
			}
			args := []string{"--user", "disable", "--now", "agora-de-shell-overlay.service"}
			if *update.DiagnosticOverlayEnabled {
				args = []string{"--user", "enable", "--now", "agora-de-shell-overlay.service"}
			}
			if output, err := exec.CommandContext(ctx, systemctl, args...).CombinedOutput(); err != nil {
				detail := strings.TrimSpace(string(output))
				if detail == "" {
					detail = err.Error()
				}
				writeJSON(response, http.StatusServiceUnavailable, map[string]string{"error": detail})
				return
			}
			writeJSON(response, http.StatusAccepted, collectSettings(ctx, systemctl, time.Now()))
		default:
			response.Header().Set("Allow", "GET, POST")
			writeJSON(response, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		}
	})
}

func collectSettings(ctx context.Context, systemctl string, now time.Time) settingsResponse {
	overlay := checkUserServiceSetting(ctx, systemctl, "agora-de-shell-overlay.service")
	return settingsResponse{
		GeneratedAtUnixMillis:    now.UnixMilli(),
		DiagnosticOverlayEnabled: overlay.Enabled,
		DiagnosticOverlay:        overlay,
	}
}

func checkUserServiceSetting(ctx context.Context, systemctl string, name string) shellServiceSetting {
	enabledState := systemctlState(ctx, systemctl, "--user", "is-enabled", name)
	activeState := systemctlState(ctx, systemctl, "--user", "is-active", name)
	return shellServiceSetting{
		Name:         name,
		Scope:        "user",
		EnabledState: enabledState,
		ActiveState:  activeState,
		Enabled:      enabledState == "enabled",
		Active:       activeState == "active",
	}
}

func systemctlState(ctx context.Context, systemctl string, args ...string) string {
	output, err := exec.CommandContext(ctx, systemctl, args...).CombinedOutput()
	state := strings.TrimSpace(string(output))
	if state == "" && err != nil {
		return "unavailable"
	}
	if state == "" {
		return "unknown"
	}
	return state
}

func providers(config Config) (catalogroute.Provider, catalogroute.LaunchProvider, surfaceroute.Provider, map[string]string, error) {
	if !config.FixtureProviders {
		return nil, nil, nil, nil, fmt.Errorf("shellui live providers are not wired yet; enable fixture providers for deployment testing")
	}

	appCatalog, err := catalogSource(config)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	apps, err := launchAwareAppViews(config, appCatalog)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	iconFiles := map[string]string{}
	apps = catalog.ApplyIconURLs(apps, iconResolver(config), func(path string) string {
		key := iconKey(path)
		iconFiles[key] = path
		return CatalogIconPathPrefix + key + "/" + filepath.Base(path)
	})
	surfaceProvider, err := surfaceProvider(config)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	catalogProvider := func(*http.Request) ([]catalog.AppView, error) {
		return apps, nil
	}
	return catalogProvider, launchProvider(config, appCatalog), surfaceProvider, iconFiles, nil
}

func iconResolver(config Config) *catalog.IconResolver {
	home := strings.TrimSpace(config.NativeLaunchHome)
	if home == "" {
		home = os.Getenv("HOME")
	}
	themeRoots := config.IconThemeRoots
	if len(themeRoots) == 0 {
		themeRoots = catalog.DefaultIconThemeRoots(home)
	}
	pixmapRoots := config.IconPixmapRoots
	if len(pixmapRoots) == 0 {
		pixmapRoots = catalog.DefaultIconPixmapRoots(home)
	}
	return catalog.NewIconResolver(themeRoots, pixmapRoots)
}

func iconKey(path string) string {
	sum := sha256.Sum256([]byte(path))
	return hex.EncodeToString(sum[:])[:24]
}

func catalogIconHandler(files map[string]string) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if !strings.HasPrefix(request.URL.Path, CatalogIconPathPrefix) {
			http.NotFound(response, request)
			return
		}
		if request.Method != http.MethodGet {
			response.Header().Set("Allow", http.MethodGet)
			writeJSON(response, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		rest := strings.TrimPrefix(request.URL.Path, CatalogIconPathPrefix)
		key, _, _ := strings.Cut(rest, "/")
		path, ok := files[key]
		if !ok || path == "" {
			http.NotFound(response, request)
			return
		}
		http.ServeFile(response, request, path)
	})
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
	source.Add(appcatalog.Entry{
		ID:         "shell-settings",
		Name:       "Settings",
		Exec:       "agora-de-shell-settings",
		Icon:       "preferences-desktop",
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
			URL:           "http://127.0.0.1:17780/shell/dist/desktop/?surface=operator",
			Title:         "Agora DE Shell Status",
			AppID:         "io.agorade.ShellStatus",
			LayerShell:    true,
			LayerRole:     "popup",
			Width:         980,
			Height:        720,
			ExclusiveZone: 96,
		},
		"shell-settings": {
			URL:           "http://127.0.0.1:17780/shell/dist/desktop/?surface=settings",
			Title:         "Agora DE Settings",
			AppID:         "io.agorade.ShellSettings",
			LayerShell:    true,
			LayerRole:     "popup",
			Width:         760,
			Height:        520,
			ExclusiveZone: 96,
		},
		"shell-launcher": {
			URL:           "http://127.0.0.1:17780/shell/dist/desktop/?surface=launcher",
			Title:         "Agora DE App Launcher",
			AppID:         "io.agorade.ShellLauncher",
			LayerShell:    true,
			LayerRole:     "popup",
			Width:         760,
			Height:        600,
			ExclusiveZone: 96,
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
		ID              string                 `json:"id"`
		Label           string                 `json:"label"`
		AppID           string                 `json:"app_id"`
		Title           string                 `json:"title"`
		Role            string                 `json:"role"`
		SurfaceKind     string                 `json:"surface_kind"`
		ParentSurfaceID string                 `json:"parent_surface_id"`
		Visible         bool                   `json:"visible"`
		Fullscreen      bool                   `json:"fullscreen"`
		Maximized       bool                   `json:"maximized"`
		Minimized       bool                   `json:"minimized"`
		OutputID        string                 `json:"output_id"`
		WorkspaceID     string                 `json:"workspace_id"`
		ZoneID          string                 `json:"zone_id"`
		LayoutMode      string                 `json:"layout_mode"`
		LayoutRole      string                 `json:"layout_role"`
		PolicyClass     string                 `json:"policy_class"`
		PolicyReason    string                 `json:"policy_reason"`
		Geometry        *surfaces.GeometryView `json:"geometry"`
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
	PolicyClass        string                 `json:"policy_class"`
	PolicyReason       string                 `json:"policy_reason"`
	ParentSurfaceID    string                 `json:"parent_surface_id"`
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
			ParentSurfaceID:    firstNonEmpty(tracked.ParentSurfaceID, tracked.Surface.ParentSurfaceID),
			OwnerUID:           tracked.Client.UID,
			Mapped:             mapped,
			Focused:            tracked.Focused,
			Visible:            tracked.Visible || tracked.Surface.Visible,
			Fullscreen:         tracked.Surface.Fullscreen,
			Maximized:          tracked.Surface.Maximized,
			Minimized:          tracked.Surface.Minimized,
			OutputID:           firstNonEmpty(tracked.OutputID, tracked.Surface.OutputID),
			WorkspaceID:        firstNonEmpty(tracked.WorkspaceID, tracked.Surface.WorkspaceID),
			ZoneID:             firstNonEmpty(tracked.ZoneID, tracked.Surface.ZoneID),
			LayoutMode:         firstNonEmpty(tracked.LayoutMode, tracked.Surface.LayoutMode),
			LayoutRole:         firstNonEmpty(tracked.LayoutRole, tracked.Surface.LayoutRole),
			PolicyClass:        firstNonEmpty(tracked.PolicyClass, tracked.Surface.PolicyClass),
			PolicyReason:       firstNonEmpty(tracked.PolicyReason, tracked.Surface.PolicyReason),
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

func defaultCompositorctlPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return "compositorctl"
	}
	return path
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
	LaunchID  string `json:"launch_id"`
	SurfaceID string `json:"surface_id"`
	Status    string `json:"status"`
	Surface   struct {
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
	surfaceID := response.SurfaceID
	if surfaceID == "" {
		surfaceID = response.Surface.Surface.ID
	}
	status := response.Status
	if status == "" {
		status = "launched"
	}
	return catalogroute.LaunchResult{
		LaunchID:  response.LaunchID,
		SurfaceID: surfaceID,
		Status:    status,
	}, nil
}

type surfaceActionRequest struct {
	Action    string `json:"action"`
	SurfaceID string `json:"surfaceId"`
	Enabled   *bool  `json:"enabled,omitempty"`
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
	enabled := "true"
	if action.Enabled != nil {
		enabled = strconv.FormatBool(*action.Enabled)
	}
	switch action.Action {
	case "focus":
		return []string{"surface", "focus", "--surface", action.SurfaceID, "--timeout-ms", "2000"}, true
	case "close":
		return []string{"surface", "close", "--surface", action.SurfaceID, "--timeout-ms", "2000"}, true
	case "maximize":
		return []string{"surface", "maximize", "--surface", action.SurfaceID, "--enabled=" + enabled, "--timeout-ms", "2000"}, true
	case "minimize":
		return []string{"surface", "minimize", "--surface", action.SurfaceID, "--enabled=" + enabled, "--timeout-ms", "2000"}, true
	case "fullscreen":
		return []string{"surface", "fullscreen", "--surface", action.SurfaceID, "--enabled=" + enabled, "--timeout-ms", "2000"}, true
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
	case "io.agorade.ShellLauncher", "io.agorade.ShellOverlay", "io.agorade.ShellSettings", "io.agorade.ShellStatus":
		return true
	default:
		return false
	}
}

type workspacesResponse struct {
	CurrentWorkspaceID string          `json:"currentWorkspaceId"`
	CurrentOutputID    string          `json:"currentOutputId,omitempty"`
	Workspaces         []workspaceView `json:"workspaces"`
}

type workspaceView struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	OutputID     string `json:"outputId,omitempty"`
	Active       bool   `json:"active"`
	SurfaceCount int    `json:"surfaceCount"`
}

type workspaceActionRequest struct {
	Action      string `json:"action"`
	WorkspaceID string `json:"workspaceId"`
	OutputID    string `json:"outputId,omitempty"`
}

type workspaceActionResponse struct {
	Action             string          `json:"action"`
	WorkspaceID        string          `json:"workspaceId"`
	CurrentWorkspaceID string          `json:"currentWorkspaceId"`
	CurrentOutputID    string          `json:"currentOutputId,omitempty"`
	Status             string          `json:"status"`
	Workspace          workspaceView   `json:"workspace"`
	Workspaces         []workspaceView `json:"workspaces"`
}

type workspaceRouteConfig struct {
	CompositorctlPath string
	UseCompositorctl  bool
	SurfaceProvider   surfaceroute.Provider
}

func workspacesHandler(config workspaceRouteConfig) http.Handler {
	path := defaultCompositorctlPath(config.CompositorctlPath)
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
		state := collectWorkspaceState(request, config.SurfaceProvider)
		if config.UseCompositorctl {
			ctx, cancel := context.WithTimeout(request.Context(), 3*time.Second)
			defer cancel()
			output, err := exec.CommandContext(ctx, path, "layout", "get").CombinedOutput()
			if err != nil {
				writeCompositorctlError(response, output, err)
				return
			}
			layoutState, err := workspaceStateFromCompositorctlLayout(output)
			if err != nil {
				writeJSON(response, http.StatusServiceUnavailable, classifiedAPIError{Error: err.Error(), ErrorClass: "invalid_response"})
				return
			}
			state = layoutState
		}
		writeJSON(response, http.StatusOK, state)
	})
}

func workspaceActionHandler(config workspaceRouteConfig) http.Handler {
	path := defaultCompositorctlPath(config.CompositorctlPath)
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
		action.Action = strings.TrimSpace(action.Action)
		action.WorkspaceID = strings.TrimSpace(action.WorkspaceID)
		action.OutputID = strings.TrimSpace(action.OutputID)
		if action.Action != "activate" {
			writeJSON(response, http.StatusBadRequest, map[string]string{"error": "unsupported workspace action"})
			return
		}
		if action.WorkspaceID == "" {
			writeJSON(response, http.StatusBadRequest, map[string]string{"error": "workspaceId is required"})
			return
		}
		if config.UseCompositorctl {
			ctx, cancel := context.WithTimeout(request.Context(), 5*time.Second)
			defer cancel()
			args := []string{"workspace", "activate", "--workspace", action.WorkspaceID, "--timeout-ms", "2000"}
			if action.OutputID != "" {
				args = []string{"workspace", "activate", "--workspace", action.WorkspaceID, "--output", action.OutputID, "--timeout-ms", "2000"}
			}
			if output, err := exec.CommandContext(ctx, path, args...).CombinedOutput(); err != nil {
				writeCompositorctlError(response, output, err)
				return
			}
		}
		state := collectWorkspaceState(request, config.SurfaceProvider)
		if config.UseCompositorctl {
			ctx, cancel := context.WithTimeout(request.Context(), 3*time.Second)
			defer cancel()
			if output, err := exec.CommandContext(ctx, path, "layout", "get").CombinedOutput(); err == nil {
				if layoutState, decodeErr := workspaceStateFromCompositorctlLayout(output); decodeErr == nil {
					state = layoutState
				}
			}
		}
		workspace := workspaceView{ID: action.WorkspaceID, Name: workspaceDisplayName(action.WorkspaceID), OutputID: action.OutputID, Active: true}
		for _, candidate := range state.Workspaces {
			if candidate.ID == action.WorkspaceID {
				workspace = candidate
				break
			}
		}
		writeJSON(response, http.StatusAccepted, workspaceActionResponse{
			Action:             action.Action,
			WorkspaceID:        action.WorkspaceID,
			CurrentWorkspaceID: state.CurrentWorkspaceID,
			CurrentOutputID:    state.CurrentOutputID,
			Status:             "accepted",
			Workspace:          workspace,
			Workspaces:         state.Workspaces,
		})
	})
}

func collectWorkspaceState(request *http.Request, surfaceProvider surfaceroute.Provider) workspacesResponse {
	surfaceCounts := map[string]int{"workspace-1": 0}
	outputByWorkspace := map[string]string{}
	if surfaceProvider != nil {
		if views, err := surfaceProvider(request); err == nil {
			for _, view := range views {
				if view.Mapped && view.SurfaceKind != "layer_shell" {
					workspaceID := firstNonEmpty(view.WorkspaceID, "workspace-1")
					surfaceCounts[workspaceID]++
					if view.OutputID != "" && outputByWorkspace[workspaceID] == "" {
						outputByWorkspace[workspaceID] = view.OutputID
					}
				}
			}
		}
	}
	workspaceIDs := make([]string, 0, len(surfaceCounts))
	for workspaceID := range surfaceCounts {
		workspaceIDs = append(workspaceIDs, workspaceID)
	}
	sortWorkspaceIDs(workspaceIDs)
	workspaces := make([]workspaceView, 0, len(workspaceIDs))
	for _, workspaceID := range workspaceIDs {
		workspaces = append(workspaces, workspaceView{
			ID:           workspaceID,
			Name:         workspaceDisplayName(workspaceID),
			OutputID:     outputByWorkspace[workspaceID],
			Active:       workspaceID == "workspace-1",
			SurfaceCount: surfaceCounts[workspaceID],
		})
	}
	return workspacesResponse{
		CurrentWorkspaceID: "workspace-1",
		CurrentOutputID:    outputByWorkspace["workspace-1"],
		Workspaces:         workspaces,
	}
}

func workspaceStateFromCompositorctlLayout(payload []byte) (workspacesResponse, error) {
	var response struct {
		Layout struct {
			Surfaces []struct {
				SurfaceID   string `json:"surface_id"`
				OutputID    string `json:"output_id"`
				WorkspaceID string `json:"workspace_id"`
				Visible     bool   `json:"visible"`
			} `json:"surfaces"`
			Workspaces []struct {
				ID           string   `json:"id"`
				Name         string   `json:"name"`
				OutputID     string   `json:"output_id"`
				Active       bool     `json:"active"`
				SurfaceOrder []string `json:"surface_order"`
			} `json:"workspaces"`
		} `json:"layout"`
	}
	if err := json.Unmarshal(payload, &response); err != nil {
		return workspacesResponse{}, fmt.Errorf("decode compositorctl workspace layout: %w", err)
	}
	counts := map[string]int{}
	outputByWorkspace := map[string]string{}
	for _, surface := range response.Layout.Surfaces {
		workspaceID := firstNonEmpty(surface.WorkspaceID, "workspace-1")
		counts[workspaceID]++
		if surface.OutputID != "" && outputByWorkspace[workspaceID] == "" {
			outputByWorkspace[workspaceID] = surface.OutputID
		}
	}
	workspaces := make([]workspaceView, 0, len(response.Layout.Workspaces))
	current := ""
	currentOutputID := ""
	seen := map[string]bool{}
	for _, workspace := range response.Layout.Workspaces {
		workspaceID := firstNonEmpty(workspace.ID, "workspace-1")
		outputID := firstNonEmpty(workspace.OutputID, outputByWorkspace[workspaceID])
		count := counts[workspaceID]
		if count == 0 && len(workspace.SurfaceOrder) > 0 {
			count = len(workspace.SurfaceOrder)
		}
		workspaces = append(workspaces, workspaceView{
			ID:           workspaceID,
			Name:         firstNonEmpty(workspace.Name, workspaceDisplayName(workspaceID)),
			OutputID:     outputID,
			Active:       workspace.Active,
			SurfaceCount: count,
		})
		seen[workspaceID] = true
		if workspace.Active {
			current = workspaceID
			currentOutputID = outputID
		}
	}
	for workspaceID, count := range counts {
		if seen[workspaceID] {
			continue
		}
		workspaces = append(workspaces, workspaceView{
			ID:           workspaceID,
			Name:         workspaceDisplayName(workspaceID),
			OutputID:     outputByWorkspace[workspaceID],
			SurfaceCount: count,
		})
	}
	if len(workspaces) == 0 {
		workspaces = []workspaceView{{ID: "workspace-1", Name: "workspace 1", Active: true}}
	}
	if current == "" {
		for index := range workspaces {
			if workspaces[index].Active {
				current = workspaces[index].ID
				break
			}
		}
	}
	if current == "" {
		current = workspaces[0].ID
		currentOutputID = workspaces[0].OutputID
		workspaces[0].Active = true
	}
	sort.SliceStable(workspaces, func(i, j int) bool {
		if workspaces[i].ID == "workspace-1" || workspaces[j].ID == "workspace-1" {
			return workspaces[i].ID == "workspace-1"
		}
		return workspaces[i].ID < workspaces[j].ID
	})
	return workspacesResponse{CurrentWorkspaceID: current, CurrentOutputID: currentOutputID, Workspaces: workspaces}, nil
}

func sortWorkspaceIDs(workspaceIDs []string) {
	sort.SliceStable(workspaceIDs, func(i, j int) bool {
		if workspaceIDs[i] == "workspace-1" || workspaceIDs[j] == "workspace-1" {
			return workspaceIDs[i] == "workspace-1"
		}
		return workspaceIDs[i] < workspaceIDs[j]
	})
}

func workspaceDisplayName(workspaceID string) string {
	if strings.HasPrefix(workspaceID, "workspace-") {
		suffix := strings.TrimSpace(strings.TrimPrefix(workspaceID, "workspace-"))
		if suffix != "" {
			return "workspace " + suffix
		}
	}
	return workspaceID
}

type operatorStatusResponse struct {
	GeneratedAtUnixMillis int64                  `json:"generatedAtUnixMillis"`
	Overall               string                 `json:"overall"`
	Services              []operatorServiceView  `json:"services"`
	Sockets               []operatorSocketView   `json:"sockets"`
	Outputs               []operatorOutputView   `json:"outputs"`
	Surfaces              operatorSurfaceSummary `json:"surfaces"`
	Timing                timingSummaryResponse  `json:"timing"`
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

func operatorStatusHandler(config Config, surfaceProvider surfaceroute.Provider, timings *timingRecorder) http.Handler {
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
		status := collectOperatorStatus(ctx, path, surfaceProvider, timings, time.Now())
		writeJSON(response, http.StatusOK, status)
	})
}

func timingDiagnosticsHandler(timings *timingRecorder) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != TimingDiagnosticsPath {
			http.NotFound(response, request)
			return
		}
		if request.Method != http.MethodGet {
			response.Header().Set("Allow", http.MethodGet)
			writeJSON(response, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		writeJSON(response, http.StatusOK, timings.summary(time.Now()))
	})
}

func collectOperatorStatus(ctx context.Context, compositorctl string, surfaceProvider surfaceroute.Provider, timings *timingRecorder, now time.Time) operatorStatusResponse {
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
		Timing:                timings.summary(now),
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

func shellAssetHandler(root string, themeCSS string) http.Handler {
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

		writeShellHTML(response, request, themeCSS)
	})
}

const (
	shellThemeCSSPlaceholder = "__AGORA_THEME_CSS__"
	shellSurfacePlaceholder  = "__AGORA_SURFACE__"
)

// writeShellHTML serves the embedded TypeScript-authored shell surface
// templates, substituting the active theme CSS and the requested surface name
// at the one serving seam. The templates themselves are generated from
// @agora-de/renderer by harness/build/generate-shell-html.mjs.
func writeShellHTML(response http.ResponseWriter, request *http.Request, themeCSS string) {
	surface := strings.TrimSpace(request.URL.Query().Get("surface"))
	if surface == "" {
		surface = "desktop"
	}
	template := "shellassets/" + shellTemplateName(surface)
	data, err := embeddedShellAssets.ReadFile(template)
	if err != nil {
		http.Error(response, "shell template not found", http.StatusInternalServerError)
		return
	}
	rendered := strings.ReplaceAll(string(data), shellThemeCSSPlaceholder, themeCSS)
	rendered = strings.ReplaceAll(rendered, shellSurfacePlaceholder, html.EscapeString(surface))
	response.Header().Set("Content-Type", "text/html; charset=utf-8")
	response.Header().Set("Content-Length", strconv.Itoa(len(rendered)))
	_, _ = response.Write([]byte(rendered))
}

func shellTemplateName(surface string) string {
	switch surface {
	case "dock", "panel":
		return "panel.html"
	case "launcher":
		return "launcher.html"
	case "operator":
		return "operator.html"
	case "settings":
		return "settings.html"
	case "overlay":
		return "overlay.html"
	case "background-fallback":
		return "background-fallback.html"
	default:
		return "background.html"
	}
}

