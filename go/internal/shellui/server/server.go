package server

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"agora-de.local/go/internal/appcatalog"
	"agora-de.local/go/internal/shellui/catalog"
	"agora-de.local/go/internal/shellui/catalogroute"
	"agora-de.local/go/internal/shellui/staticserve"
	"agora-de.local/go/internal/shellui/surfaceroute"
	"agora-de.local/go/internal/shellui/surfaces"
)

const (
	DefaultListenAddress = "127.0.0.1:7780"
	WorkControlsPath     = "/api/work-surface-controls"
	SurfaceActionPath    = "/api/surfaces/action"

	SurfaceProviderFixture       = "fixture"
	SurfaceProviderCompositorctl = "compositorctl"
)

type Config struct {
	StaticRoot        string
	FixtureProviders  bool
	SurfaceProvider   string
	CompositorctlPath string
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
	mux.Handle("/shell/dist/", shellAssetHandler(config.StaticRoot))
	return mux, nil
}

func providers(config Config) (catalogroute.Provider, catalogroute.LaunchProvider, surfaceroute.Provider, error) {
	if !config.FixtureProviders {
		return nil, nil, nil, fmt.Errorf("shellui live providers are not wired yet; enable fixture providers for deployment testing")
	}

	appCatalog := fixtureCatalog()
	apps := catalog.VisibleAppViews(appCatalog)
	surfaceProvider, err := surfaceProvider(config)
	if err != nil {
		return nil, nil, nil, err
	}

	catalogProvider := func(*http.Request) ([]catalog.AppView, error) {
		return apps, nil
	}
	return catalogProvider, launchProvider(config, appCatalog), surfaceProvider, nil
}

func fixtureCatalog() *appcatalog.Catalog {
	source := appcatalog.NewCatalog()
	source.Add(appcatalog.Entry{
		ID:   "example-browser",
		Name: "Example Browser",
		Exec: "example-browser --new-window %u",
		Icon: "example-browser",
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
	}
}

func launchProvider(config Config, appCatalog *appcatalog.Catalog) catalogroute.LaunchProvider {
	path := strings.TrimSpace(config.CompositorctlPath)
	if path == "" {
		path = "compositorctl"
	}
	targets := launchTargets()
	return func(request *http.Request, launch catalogroute.LaunchRequest) (catalogroute.LaunchResult, error) {
		entry, ok := appCatalog.Get(launch.AppID)
		if !ok || entry.NoDisplay {
			return catalogroute.LaunchResult{}, fmt.Errorf("app %q not found", launch.AppID)
		}
		target, ok := targets[launch.AppID]
		if !ok {
			return catalogroute.LaunchResult{}, fmt.Errorf("app %q is not launchable by shellui", launch.AppID)
		}
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
		result.AppID = launch.AppID
		if result.Status == "" {
			result.Status = "launched"
		}
		return result, nil
	}
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

func writeJSON(response http.ResponseWriter, status int, value any) {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(value)
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
    html,
    body {
      background: #f8fafc;
      color: #102027;
      font: 600 22px system-ui, sans-serif;
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
      background: #00d1b2;
      border-radius: 4px;
      height: 40px;
      width: 40px;
    }
    .taskbar {
      align-items: center;
      background: #f8fafc;
      border-top: 4px solid #00d1b2;
      box-shadow: inset 0 1px 0 #cbd5e1;
      box-sizing: border-box;
      display: flex;
      gap: 18px;
      min-height: 96px;
      padding: 0 28px;
    }
    .badge {
      align-items: center;
      background: #102027;
      border-radius: 4px;
      color: #f8fafc;
      display: inline-flex;
      height: 44px;
      justify-content: center;
      min-width: 132px;
      padding: 0 16px;
    }
    .slot {
      align-items: center;
      border: 2px solid #94a3b8;
      border-radius: 4px;
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
</html>`, rows, bodyClass, escapedSurface, escapedSurface, taskbarHTML)
}

func writePanelHTML(response http.ResponseWriter, surface string) {
	escapedSurface := html.EscapeString(surface)
	fmt.Fprintf(response, `<!doctype html>
<html>
<head>
  <title>agora-de shell panel</title>
  <meta name="color-scheme" content="light">
  <style>
    html,
    body {
      background: #f8fafc !important;
      color: #102027;
      height: 100%%;
      margin: 0;
      overflow: hidden;
      width: 100%%;
    }
    body {
      align-items: stretch;
      box-sizing: border-box;
      display: flex;
      font: 600 18px system-ui, sans-serif;
    }
    .panel {
      align-items: center;
      background: #f8fafc;
      border-top: 4px solid #00d1b2;
      box-shadow: inset 0 1px 0 #cbd5e1;
      box-sizing: border-box;
      display: flex;
      gap: 14px;
      min-height: 96px;
      padding: 0 22px;
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
      border-radius: 4px;
      display: inline-flex;
      height: 44px;
      justify-content: center;
      padding: 0 16px;
      white-space: nowrap;
    }
    .brand {
      background: #102027;
      color: #f8fafc;
      min-width: 132px;
    }
    .control {
      background: #00d1b2;
      border: 0;
      color: #102027;
      min-width: 94px;
    }
    .control.secondary {
      background: #ffffff;
      border: 2px solid #94a3b8;
    }
    .workspace,
    .status,
    .clock {
      border: 2px solid #94a3b8;
      color: #102027;
    }
    .dock-section {
      align-items: center;
      display: flex;
      gap: 10px;
      min-width: 0;
    }
    .apps {
      flex: 0 1 520px;
      overflow: hidden;
    }
    .running {
      flex: 1 1 auto;
      overflow: hidden;
    }
    .dock-item {
      align-items: center;
      background: #ffffff;
      border: 2px solid #cbd5e1;
      border-radius: 4px;
      color: #102027;
      display: inline-flex;
      height: 44px;
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
      border-color: #00d1b2;
      box-shadow: inset 0 -3px 0 #00d1b2;
    }
    .surface-actions {
      align-items: center;
      display: inline-flex;
      gap: 6px;
    }
    .surface-action {
      background: #ffffff;
      border: 2px solid #94a3b8;
      border-radius: 4px;
      color: #102027;
      height: 44px;
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
      border-color: #00d1b2;
    }
    .status.warn {
      border-color: #eab308;
    }
    .muted {
      color: #475569;
    }
  </style>
</head>
<body data-surface="%s">
  <main class="panel" aria-label="Agora DE shell panel">
    <span class="brand">agora-de</span>
    <button class="control" id="apps-button" type="button">Apps</button>
    <button class="control secondary" id="refresh-button" type="button">Refresh</button>
    <section class="dock-section apps" id="apps-list" aria-label="Applications">
      <span class="dock-item muted">loading apps</span>
    </section>
    <section class="dock-section running" id="running-list" aria-label="Running surfaces">
      <span class="dock-item muted">loading surfaces</span>
    </section>
    <span class="workspace" id="workspace-label">workspace 1</span>
    <span class="status" id="status-label">starting</span>
    <time class="clock" id="clock-label">--:--</time>
  </main>
  <script>
    const state = {
      apps: [],
      surfaces: [],
      surface: %q
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

    function renderList(id, emptyLabel, values, mapper) {
      const target = document.getElementById(id);
      target.replaceChildren();
      if (!values.length) {
        target.appendChild(item(emptyLabel, "muted"));
        return;
      }
      values.slice(0, 4).forEach((value) => target.appendChild(mapper(value)));
    }

    function render() {
      renderList("apps-list", "no apps", state.apps, (app) => {
        const appButton = button(text(app.name, app.id), "dock-item", () => launchApp(app.id));
        appButton.disabled = !app.launchable;
        return appButton;
      });
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
      status.textContent = workSurfaces.length ? workSurfaces.length + " running" : "ready";
      status.className = "status " + (workSurfaces.length ? "ready" : "warn");
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
        const [catalog, surfaces] = await Promise.all([
          loadJSON("/api/catalog/apps"),
          loadJSON("/api/surfaces")
        ]);
        state.apps = Array.isArray(catalog.apps) ? catalog.apps : [];
        state.surfaces = Array.isArray(surfaces.surfaces) ? surfaces.surfaces : [];
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

    function updateClock() {
      const now = new Date();
      document.getElementById("clock-label").textContent = now.toLocaleTimeString([], {
        hour: "2-digit",
        minute: "2-digit"
      });
    }

    document.getElementById("apps-button").addEventListener("click", refresh);
    document.getElementById("refresh-button").addEventListener("click", refresh);
    updateClock();
    refresh();
    setInterval(updateClock, 30000);
    setInterval(refresh, 3000);
  </script>
</body>
</html>`, escapedSurface, escapedSurface)
}
