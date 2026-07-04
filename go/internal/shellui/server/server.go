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
	catalogProvider, surfaceProvider, err := providers(config)
	if err != nil {
		return nil, err
	}

	mux := http.NewServeMux()
	mux.Handle(catalogroute.AppsPath, catalogroute.New(catalogProvider))
	mux.Handle(surfaceroute.SurfacesPath, surfaceroute.New(surfaceProvider))
	mux.Handle(WorkControlsPath, surfaceroute.Handler{
		Path:     WorkControlsPath,
		Provider: surfaceProvider,
	})
	mux.Handle("/shell/dist/", shellAssetHandler(config.StaticRoot))
	return mux, nil
}

func providers(config Config) (catalogroute.Provider, surfaceroute.Provider, error) {
	if !config.FixtureProviders {
		return nil, nil, fmt.Errorf("shellui live providers are not wired yet; enable fixture providers for deployment testing")
	}

	apps := catalog.VisibleAppViews(fixtureCatalog())
	surfaceProvider, err := surfaceProvider(config)
	if err != nil {
		return nil, nil, err
	}

	return func(*http.Request) ([]catalog.AppView, error) {
		return apps, nil
	}, surfaceProvider, nil
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
		ID      string `json:"id"`
		Visible bool   `json:"visible"`
	} `json:"surface"`
	Client struct {
		UID int `json:"uid"`
	} `json:"client"`
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
      font: 600 20px system-ui, sans-serif;
    }
    .panel {
      align-items: center;
      background: #f8fafc;
      border-top: 4px solid #00d1b2;
      box-shadow: inset 0 1px 0 #cbd5e1;
      box-sizing: border-box;
      display: flex;
      gap: 18px;
      min-height: 96px;
      padding: 0 28px;
      width: 100vw;
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
<body data-surface="%s">
  <main class="panel" aria-label="Agora DE shell panel">
    <span class="badge">agora-de</span>
    <span class="slot">shell: %s</span>
    <span class="slot">workspace 1</span>
    <span class="slot">ready</span>
  </main>
</body>
</html>`, escapedSurface, escapedSurface)
}
