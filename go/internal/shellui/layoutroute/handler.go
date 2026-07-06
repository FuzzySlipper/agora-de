package layoutroute

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"agora-de.local/go/internal/shellui/surfaceroute"
	"agora-de.local/go/internal/shellui/surfaces"
)

const LayoutPath = "/api/layout"
const ActionPath = "/api/layout/action"

type Config struct {
	CompositorctlPath string
	UseCompositorctl  bool
	SurfaceProvider   surfaceroute.Provider
}

type layoutResponse struct {
	Layout layoutState `json:"layout"`
}

type layoutState struct {
	Mode       string            `json:"mode"`
	Revision   uint64            `json:"revision"`
	Settings   layoutSettings    `json:"settings"`
	Surfaces   []layoutSurface   `json:"surfaces"`
	Workspaces []layoutWorkspace `json:"workspaces"`
}

type layoutSettings struct {
	Rule        string     `json:"rule"`
	Mode        string     `json:"mode"`
	Gaps        layoutGaps `json:"gaps"`
	MasterCount int        `json:"masterCount"`
	MasterRatio float64    `json:"masterRatio"`
	SmartGaps   bool       `json:"smartGaps"`
}

type layoutGaps struct {
	OuterHorizontal int `json:"outerHorizontal"`
	OuterVertical   int `json:"outerVertical"`
	InnerHorizontal int `json:"innerHorizontal"`
	InnerVertical   int `json:"innerVertical"`
}

type layoutSurface struct {
	SurfaceID     string                 `json:"surfaceId"`
	Label         string                 `json:"label"`
	AppID         string                 `json:"appId,omitempty"`
	Title         string                 `json:"title,omitempty"`
	Role          string                 `json:"role,omitempty"`
	OutputID      string                 `json:"outputId,omitempty"`
	WorkspaceID   string                 `json:"workspaceId"`
	ZoneID        string                 `json:"zoneId"`
	Mode          string                 `json:"mode"`
	Participation string                 `json:"participation"`
	Floating      bool                   `json:"floating"`
	Focused       bool                   `json:"focused"`
	Visible       bool                   `json:"visible"`
	Geometry      *surfaces.GeometryView `json:"geometry,omitempty"`
	Order         int                    `json:"order"`
}

type layoutZone struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Kind       string   `json:"kind"`
	SurfaceIDs []string `json:"surfaceIds"`
}

type layoutWorkspace struct {
	ID           string       `json:"id"`
	Name         string       `json:"name"`
	OutputID     string       `json:"outputId,omitempty"`
	Active       bool         `json:"active"`
	Zones        []layoutZone `json:"zones"`
	SurfaceOrder []string     `json:"surfaceOrder"`
}

type compositorctlLayoutResponse struct {
	Layout struct {
		Mode     string                      `json:"mode"`
		Revision uint64                      `json:"revision"`
		Settings compositorctlLayoutSettings `json:"settings"`
		Surfaces []struct {
			SurfaceID     string                 `json:"surface_id"`
			Label         string                 `json:"label"`
			AppID         string                 `json:"app_id"`
			Title         string                 `json:"title"`
			Role          string                 `json:"role"`
			OutputID      string                 `json:"output_id"`
			WorkspaceID   string                 `json:"workspace_id"`
			ZoneID        string                 `json:"zone_id"`
			Mode          string                 `json:"mode"`
			Participation string                 `json:"participation"`
			Floating      bool                   `json:"floating"`
			Focused       bool                   `json:"focused"`
			Visible       bool                   `json:"visible"`
			Geometry      *surfaces.GeometryView `json:"geometry"`
			Order         int                    `json:"order"`
		} `json:"surfaces"`
		Workspaces []struct {
			ID       string `json:"id"`
			Name     string `json:"name"`
			OutputID string `json:"output_id"`
			Active   bool   `json:"active"`
			Zones    []struct {
				ID         string   `json:"id"`
				Name       string   `json:"name"`
				Kind       string   `json:"kind"`
				SurfaceIDs []string `json:"surface_ids"`
			} `json:"zones"`
			SurfaceOrder []string `json:"surface_order"`
		} `json:"workspaces"`
	} `json:"layout"`
}

type compositorctlLayoutSettings struct {
	Rule string `json:"rule"`
	Mode string `json:"mode"`
	Gaps struct {
		OuterHorizontal int `json:"outer_horizontal"`
		OuterVertical   int `json:"outer_vertical"`
		InnerHorizontal int `json:"inner_horizontal"`
		InnerVertical   int `json:"inner_vertical"`
	} `json:"gaps"`
	MasterCount int     `json:"master_count"`
	MasterRatio float64 `json:"master_ratio"`
	SmartGaps   bool    `json:"smart_gaps"`
}

type actionRequest struct {
	Action      string                 `json:"action"`
	Mode        string                 `json:"mode"`
	SurfaceID   string                 `json:"surfaceId"`
	WorkspaceID string                 `json:"workspaceId"`
	ZoneID      string                 `json:"zoneId"`
	Geometry    *surfaces.GeometryView `json:"geometry"`
	Floating    *bool                  `json:"floating"`
	Enabled     *bool                  `json:"enabled"`
}

type actionResponse struct {
	Action      string `json:"action"`
	SurfaceID   string `json:"surfaceId,omitempty"`
	WorkspaceID string `json:"workspaceId,omitempty"`
	ZoneID      string `json:"zoneId,omitempty"`
	Mode        string `json:"mode,omitempty"`
	Status      string `json:"status"`
}

type errorResponse struct {
	Error      string `json:"error"`
	ErrorClass string `json:"errorClass,omitempty"`
}

func New(config Config) http.Handler {
	path := compositorctlPath(config.CompositorctlPath)
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != LayoutPath {
			http.NotFound(response, request)
			return
		}
		if request.Method != http.MethodGet {
			response.Header().Set("Allow", http.MethodGet)
			writeJSON(response, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
			return
		}
		if config.UseCompositorctl {
			ctx, cancel := context.WithTimeout(request.Context(), 3*time.Second)
			defer cancel()
			output, err := exec.CommandContext(ctx, path, "layout", "get").CombinedOutput()
			if err != nil {
				errorClass, _ := parseCompositorctlError(strings.TrimSpace(string(output)))
				if errorClass == "backend_unsupported" && config.SurfaceProvider != nil {
					writeJSON(response, http.StatusOK, layoutResponse{Layout: collectLayoutState(request, config.SurfaceProvider)})
					return
				}
				writeCompositorctlError(response, output, err)
				return
			}
			layout, err := decodeCompositorctlLayout(output)
			if err != nil {
				writeJSON(response, http.StatusServiceUnavailable, errorResponse{Error: err.Error(), ErrorClass: "invalid_response"})
				return
			}
			writeJSON(response, http.StatusOK, layoutResponse{Layout: layout})
			return
		}
		writeJSON(response, http.StatusOK, layoutResponse{Layout: collectLayoutState(request, config.SurfaceProvider)})
	})
}

func NewAction(config Config) http.Handler {
	path := compositorctlPath(config.CompositorctlPath)
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != ActionPath {
			http.NotFound(response, request)
			return
		}
		if request.Method != http.MethodPost {
			response.Header().Set("Allow", http.MethodPost)
			writeJSON(response, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
			return
		}
		var action actionRequest
		if err := json.NewDecoder(request.Body).Decode(&action); err != nil {
			writeJSON(response, http.StatusBadRequest, errorResponse{Error: "invalid layout action request"})
			return
		}
		args, err := actionArgs(action)
		if err != nil {
			writeJSON(response, http.StatusBadRequest, errorResponse{Error: err.Error()})
			return
		}
		ctx, cancel := context.WithTimeout(request.Context(), 5*time.Second)
		defer cancel()
		if output, err := exec.CommandContext(ctx, path, args...).CombinedOutput(); err != nil {
			writeCompositorctlError(response, output, err)
			return
		}
		writeJSON(response, http.StatusAccepted, actionResponse{
			Action:      action.Action,
			SurfaceID:   strings.TrimSpace(action.SurfaceID),
			WorkspaceID: strings.TrimSpace(action.WorkspaceID),
			ZoneID:      strings.TrimSpace(action.ZoneID),
			Mode:        strings.TrimSpace(action.Mode),
			Status:      "accepted",
		})
	})
}

func actionArgs(action actionRequest) ([]string, error) {
	switch action.Action {
	case "setMode":
		mode := strings.TrimSpace(action.Mode)
		if mode == "" {
			return nil, fmt.Errorf("mode is required")
		}
		return []string{"layout", "set-mode", "--mode", mode}, nil
	case "assignZone":
		return zoneActionArgs("assign-zone", action)
	case "promote":
		surfaceID := strings.TrimSpace(action.SurfaceID)
		if surfaceID == "" {
			return nil, fmt.Errorf("surfaceId is required")
		}
		return []string{"surface", "promote", "--surface", surfaceID, "--timeout-ms", "2000"}, nil
	case "tile":
		return zoneActionArgs("tile", action)
	case "moveResize":
		surfaceID := strings.TrimSpace(action.SurfaceID)
		if surfaceID == "" {
			return nil, fmt.Errorf("surfaceId is required")
		}
		if action.Geometry == nil || action.Geometry.Width <= 0 || action.Geometry.Height <= 0 {
			return nil, fmt.Errorf("geometry width and height must be positive")
		}
		return []string{
			"surface", "move-resize",
			"--surface", surfaceID,
			"--x", fmt.Sprint(action.Geometry.X),
			"--y", fmt.Sprint(action.Geometry.Y),
			"--width", fmt.Sprint(action.Geometry.Width),
			"--height", fmt.Sprint(action.Geometry.Height),
			"--timeout-ms", "2000",
		}, nil
	case "setFloating":
		surfaceID := strings.TrimSpace(action.SurfaceID)
		if surfaceID == "" {
			return nil, fmt.Errorf("surfaceId is required")
		}
		enabled := true
		if action.Floating != nil {
			enabled = *action.Floating
		} else if action.Enabled != nil {
			enabled = *action.Enabled
		}
		return []string{"surface", "set-floating", "--surface", surfaceID, fmt.Sprintf("--enabled=%t", enabled), "--timeout-ms", "2000"}, nil
	case "activateWorkspace":
		workspaceID := strings.TrimSpace(action.WorkspaceID)
		if workspaceID == "" {
			return nil, fmt.Errorf("workspaceId is required")
		}
		return []string{"workspace", "activate", "--workspace", workspaceID, "--timeout-ms", "2000"}, nil
	default:
		return nil, fmt.Errorf("unsupported layout action")
	}
}

func zoneActionArgs(command string, action actionRequest) ([]string, error) {
	surfaceID := strings.TrimSpace(action.SurfaceID)
	zoneID := strings.TrimSpace(action.ZoneID)
	if surfaceID == "" {
		return nil, fmt.Errorf("surfaceId is required")
	}
	if zoneID == "" {
		return nil, fmt.Errorf("zoneId is required")
	}
	args := []string{"surface", command, "--surface", surfaceID, "--zone", zoneID}
	if workspaceID := strings.TrimSpace(action.WorkspaceID); workspaceID != "" {
		args = append(args, "--workspace", workspaceID)
	}
	if action.Geometry != nil {
		if action.Geometry.Width <= 0 || action.Geometry.Height <= 0 {
			return nil, fmt.Errorf("geometry width and height must be positive")
		}
		args = append(args,
			"--x", fmt.Sprint(action.Geometry.X),
			"--y", fmt.Sprint(action.Geometry.Y),
			"--width", fmt.Sprint(action.Geometry.Width),
			"--height", fmt.Sprint(action.Geometry.Height),
		)
	}
	args = append(args, "--timeout-ms", "2000")
	return args, nil
}

func decodeCompositorctlLayout(payload []byte) (layoutState, error) {
	var response compositorctlLayoutResponse
	if err := json.Unmarshal(payload, &response); err != nil {
		return layoutState{}, fmt.Errorf("decode compositorctl layout: %w", err)
	}
	state := layoutState{
		Mode:       firstNonEmpty(response.Layout.Mode, "freeform"),
		Revision:   response.Layout.Revision,
		Settings:   layoutSettingsFromCompositorctl(response.Layout.Settings, response.Layout.Mode),
		Surfaces:   make([]layoutSurface, 0, len(response.Layout.Surfaces)),
		Workspaces: make([]layoutWorkspace, 0, len(response.Layout.Workspaces)),
	}
	for _, surface := range response.Layout.Surfaces {
		state.Surfaces = append(state.Surfaces, layoutSurface{
			SurfaceID:     surface.SurfaceID,
			Label:         surface.Label,
			AppID:         surface.AppID,
			Title:         surface.Title,
			Role:          surface.Role,
			OutputID:      surface.OutputID,
			WorkspaceID:   firstNonEmpty(surface.WorkspaceID, "workspace-1"),
			ZoneID:        firstNonEmpty(surface.ZoneID, "primary"),
			Mode:          firstNonEmpty(surface.Mode, state.Mode),
			Participation: firstNonEmpty(surface.Participation, "floating"),
			Floating:      surface.Floating,
			Focused:       surface.Focused,
			Visible:       surface.Visible,
			Geometry:      surface.Geometry,
			Order:         surface.Order,
		})
	}
	for _, workspace := range response.Layout.Workspaces {
		zones := make([]layoutZone, 0, len(workspace.Zones))
		for _, zone := range workspace.Zones {
			zones = append(zones, layoutZone{
				ID:         zone.ID,
				Name:       zone.Name,
				Kind:       zone.Kind,
				SurfaceIDs: zone.SurfaceIDs,
			})
		}
		state.Workspaces = append(state.Workspaces, layoutWorkspace{
			ID:           workspace.ID,
			Name:         workspace.Name,
			OutputID:     workspace.OutputID,
			Active:       workspace.Active,
			Zones:        zones,
			SurfaceOrder: workspace.SurfaceOrder,
		})
	}
	if len(state.Workspaces) == 0 {
		state.Workspaces = []layoutWorkspace{workspaceFromSurfaces(state.Surfaces)}
	}
	return state, nil
}

func layoutSettingsFromCompositorctl(settings compositorctlLayoutSettings, fallbackMode string) layoutSettings {
	result := layoutSettings{
		Rule:        firstNonEmpty(settings.Rule, "master_stack"),
		Mode:        firstNonEmpty(settings.Mode, fallbackMode, "zones"),
		MasterCount: settings.MasterCount,
		MasterRatio: settings.MasterRatio,
		SmartGaps:   settings.SmartGaps,
		Gaps: layoutGaps{
			OuterHorizontal: settings.Gaps.OuterHorizontal,
			OuterVertical:   settings.Gaps.OuterVertical,
			InnerHorizontal: settings.Gaps.InnerHorizontal,
			InnerVertical:   settings.Gaps.InnerVertical,
		},
	}
	if result.MasterCount <= 0 {
		result.MasterCount = 1
	}
	if result.MasterRatio <= 0 {
		result.MasterRatio = 0.5
	}
	return result
}

func collectLayoutState(request *http.Request, surfaceProvider surfaceroute.Provider) layoutState {
	var views []surfaces.SurfaceView
	if surfaceProvider != nil {
		if surfaceViews, err := surfaceProvider(request); err == nil {
			views = surfaceViews
		}
	}
	layoutSurfaces := make([]layoutSurface, 0, len(views))
	for _, view := range views {
		if !view.Mapped || view.SurfaceKind == "layer_shell" {
			continue
		}
		order := len(layoutSurfaces)
		label := firstNonEmpty(view.Label, fmt.Sprintf("%d", order+1))
		mode := firstNonEmpty(view.LayoutMode, "freeform")
		participation := firstNonEmpty(view.LayoutRole, "floating")
		layoutSurfaces = append(layoutSurfaces, layoutSurface{
			SurfaceID:     view.ID,
			Label:         label,
			AppID:         view.AppID,
			Title:         view.Title,
			Role:          view.Role,
			OutputID:      view.OutputID,
			WorkspaceID:   firstNonEmpty(view.WorkspaceID, "workspace-1"),
			ZoneID:        firstNonEmpty(view.ZoneID, "primary"),
			Mode:          mode,
			Participation: participation,
			Floating:      participation == "floating",
			Focused:       view.Focused,
			Visible:       view.Visible || view.Mapped,
			Geometry:      view.Geometry,
			Order:         order,
		})
	}
	return layoutState{
		Mode:       "freeform",
		Revision:   0,
		Settings:   defaultLayoutSettings("freeform"),
		Surfaces:   layoutSurfaces,
		Workspaces: []layoutWorkspace{workspaceFromSurfaces(layoutSurfaces)},
	}
}

func defaultLayoutSettings(mode string) layoutSettings {
	return layoutSettings{
		Rule:        "master_stack",
		Mode:        firstNonEmpty(mode, "zones"),
		Gaps:        layoutGaps{},
		MasterCount: 1,
		MasterRatio: 0.5,
		SmartGaps:   true,
	}
}

func workspaceFromSurfaces(layoutSurfaces []layoutSurface) layoutWorkspace {
	zonesByID := map[string]*layoutZone{
		"primary":   {ID: "primary", Name: "Primary", Kind: "work"},
		"secondary": {ID: "secondary", Name: "Secondary", Kind: "work"},
		"transient": {ID: "transient", Name: "Transient", Kind: "floating"},
	}
	zoneOrder := []string{"primary", "secondary", "transient"}
	surfaceOrder := make([]string, 0, len(layoutSurfaces))
	outputID := ""
	for _, surface := range layoutSurfaces {
		zoneID := firstNonEmpty(surface.ZoneID, "primary")
		if _, ok := zonesByID[zoneID]; !ok {
			zonesByID[zoneID] = &layoutZone{ID: zoneID, Name: zoneID, Kind: "work"}
			zoneOrder = append(zoneOrder, zoneID)
		}
		zonesByID[zoneID].SurfaceIDs = append(zonesByID[zoneID].SurfaceIDs, surface.SurfaceID)
		surfaceOrder = append(surfaceOrder, surface.SurfaceID)
		if outputID == "" {
			outputID = surface.OutputID
		}
	}
	zones := make([]layoutZone, 0, len(zoneOrder))
	for _, zoneID := range zoneOrder {
		zones = append(zones, *zonesByID[zoneID])
	}
	return layoutWorkspace{
		ID:           "workspace-1",
		Name:         "workspace 1",
		OutputID:     outputID,
		Active:       true,
		Zones:        zones,
		SurfaceOrder: surfaceOrder,
	}
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
	writeJSON(response, status, errorResponse{Error: cleanMessage, ErrorClass: errorClass})
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func compositorctlPath(path string) string {
	if strings.TrimSpace(path) == "" {
		return "compositorctl"
	}
	return path
}

func writeJSON(response http.ResponseWriter, status int, value any) {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(value)
}
