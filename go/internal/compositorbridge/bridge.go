package compositorbridge

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"log"
	"net"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	MethodListSurfaces         = "list_surfaces"
	MethodListOutputs          = "list_outputs"
	MethodCaptureOutput        = "capture_output"
	MethodGetLayout            = "get_layout"
	MethodSetLayoutMode        = "set_layout_mode"
	MethodUpdateLayoutSettings = "update_layout_settings"
	MethodFocusSurface         = "focus_surface"
	MethodCloseSurface         = "close_surface"
	MethodMoveResizeSurface    = "move_resize_surface"
	MethodTileSurface          = "tile_surface"
	MethodSetSurfaceFloating   = "set_surface_floating"
	MethodAssignSurfaceZone    = "assign_surface_zone"
	MethodPromoteSurface       = "promote_surface"
	MethodMaximizeSurface      = "maximize_surface"
	MethodMinimizeSurface      = "minimize_surface"
	MethodFullscreenSurface    = "fullscreen_surface"
	MethodActivateWorkspace    = "activate_workspace"
)

const defaultWorkspaceID = "workspace-1"

const (
	PluginSurfaceEvent         = "surface_event"
	PluginLayoutState          = "layout_state"
	PluginFocusSurface         = "focus_surface"
	PluginFocusResponse        = "focus_response"
	PluginPlaceSurface         = "place_surface"
	PluginPlaceResponse        = "place_response"
	PluginCloseSurface         = "close_surface"
	PluginSetSurfaceState      = "set_surface_state"
	PluginSurfaceStateResponse = "surface_state_response"
	PluginPolicyReplace        = "policy_replace"
	PluginInputContext         = "input_context"
	EventMapped                = "mapped"
	EventUnmapped              = "unmapped"
	EventFocused               = "focused"
	EventFrameDone             = "frame_done"
	EventContentCommit         = "content_committed"
	EventMinimized             = "minimized"
	SurfaceKindLayer           = "layer_shell"
	SurfaceKindXDG             = "xdg_view"
	DecisionAccepted           = "accepted"
	ErrorSurfaceNotFound       = "surface_not_found"
	ErrorSurfaceStale          = "surface_stale"
	ErrorCaptureDenied         = "capture_denied"
	ErrorBackendUnsupported    = "backend_unsupported"
	ErrorCompositorUnavailable = "compositor_unavailable"
	ErrorProtocol              = "protocol_error"
	ErrorFrameTimeout          = "frame_timeout"
)

const deadClientPruneAfter = 2 * time.Second

type Config struct {
	AllowedPluginUID   uint32
	LayoutSettingsPath string
}

type Bridge struct {
	allowedPluginUID uint32

	mu                 sync.RWMutex
	plugin             *pluginSession
	pluginSeq          uint64
	surfaces           map[string]TrackedSurface
	stale              map[string]time.Time
	focusSeq           uint64
	focusWaiters       map[string]chan pluginResponse
	focusWaiterSession map[string]uint64
	placeSeq           uint64
	placeWaiters       map[string]chan pluginResponse
	placeWaiterSession map[string]uint64
	stateSeq           uint64
	stateWaiters       map[string]chan pluginResponse
	stateWaiterSession map[string]uint64
	captureSeq         uint64
	layoutSeq          uint64
	layoutMode         LayoutMode
	layoutSettings     LayoutSettings
	layoutSettingsPath string
	backendLayout      *LayoutState
	promotedSurfaceID  string
	activeWorkspaceID  string
	workspaces         map[string]workspaceRecord
	workspaceOrder     []string

	autoLayoutSeq     uint64
	autoLayoutRunning bool
}

type workspaceRecord struct {
	ID       string
	Name     string
	OutputID string
}

func New(config Config) *Bridge {
	settings, err := LoadLayoutSettings(config.LayoutSettingsPath)
	if err != nil {
		log.Printf("load layout settings: %v", err)
		settings = DefaultLayoutSettings()
	}
	return &Bridge{
		allowedPluginUID:   config.AllowedPluginUID,
		surfaces:           map[string]TrackedSurface{},
		stale:              map[string]time.Time{},
		focusWaiters:       map[string]chan pluginResponse{},
		focusWaiterSession: map[string]uint64{},
		placeWaiters:       map[string]chan pluginResponse{},
		placeWaiterSession: map[string]uint64{},
		stateWaiters:       map[string]chan pluginResponse{},
		stateWaiterSession: map[string]uint64{},
		layoutMode:         settings.Mode,
		layoutSettings:     settings,
		layoutSettingsPath: config.LayoutSettingsPath,
		activeWorkspaceID:  defaultWorkspaceID,
		workspaces: map[string]workspaceRecord{
			defaultWorkspaceID: {ID: defaultWorkspaceID, Name: workspaceDisplayName(defaultWorkspaceID)},
		},
		workspaceOrder: []string{defaultWorkspaceID},
	}
}

func (bridge *Bridge) HandlePluginConn(conn net.Conn) {
	defer conn.Close()
	if !bridge.pluginPeerAllowed(conn) {
		log.Printf("rejected compositor plugin peer")
		return
	}

	session := &pluginSession{conn: conn, enc: json.NewEncoder(conn)}
	previous := bridge.installPlugin(session)
	if previous != nil {
		_ = previous.Close()
	}
	defer bridge.clearPlugin(session)

	if err := session.Send(map[string]any{"type": PluginPolicyReplace, "surfaces": []any{}}); err != nil {
		log.Printf("send policy_replace: %v", err)
		return
	}
	if err := session.Send(map[string]any{"type": PluginInputContext}); err != nil {
		log.Printf("send input_context: %v", err)
		return
	}

	decoder := json.NewDecoder(conn)
	for {
		var event pluginEvent
		if err := decoder.Decode(&event); err != nil {
			if !errors.Is(err, io.EOF) {
				log.Printf("decode plugin event: %v", err)
			}
			return
		}
		bridge.handlePluginEvent(event)
	}
}

func (bridge *Bridge) HandleControlConn(conn net.Conn) {
	defer conn.Close()
	var request Request
	if err := json.NewDecoder(conn).Decode(&request); err != nil {
		writeResponse(conn, Response{OK: false, ErrorClass: ErrorProtocol, ErrorMessage: "decode request: " + err.Error()})
		return
	}
	body, err := bridge.Dispatch(request)
	if err != nil {
		class, message := classifyError(err)
		writeResponse(conn, Response{OK: false, ErrorClass: class, ErrorMessage: message})
		return
	}
	writeResponse(conn, Response{OK: true, Body: body})
}

func (bridge *Bridge) Dispatch(request Request) (json.RawMessage, error) {
	switch request.Method {
	case MethodListSurfaces:
		return marshalBody(ListSurfacesResponse{Surfaces: bridge.ListSurfaces()})
	case MethodListOutputs:
		return marshalBody(ListOutputsResponse{Outputs: bridge.ListOutputs()})
	case MethodCaptureOutput:
		var body CaptureOutputRequest
		if err := decodeBody(request.Body, &body); err != nil {
			return nil, err
		}
		response, err := bridge.CaptureOutput(body)
		if err != nil {
			return nil, err
		}
		return marshalBody(response)
	case MethodGetLayout:
		return marshalBody(bridge.GetLayout())
	case MethodSetLayoutMode:
		var body SetLayoutModeRequest
		if err := decodeBody(request.Body, &body); err != nil {
			return nil, err
		}
		response, err := bridge.SetLayoutMode(body)
		if err != nil {
			return nil, err
		}
		return marshalBody(response)
	case MethodUpdateLayoutSettings:
		var body UpdateLayoutSettingsRequest
		if err := decodeBody(request.Body, &body); err != nil {
			return nil, err
		}
		response, err := bridge.UpdateLayoutSettings(body)
		if err != nil {
			return nil, err
		}
		return marshalBody(response)
	case MethodFocusSurface:
		var body FocusSurfaceRequest
		if err := decodeBody(request.Body, &body); err != nil {
			return nil, err
		}
		response, err := bridge.FocusSurface(body)
		if err != nil {
			return nil, err
		}
		return marshalBody(response)
	case MethodCloseSurface:
		var body CloseSurfaceRequest
		if err := decodeBody(request.Body, &body); err != nil {
			return nil, err
		}
		response, err := bridge.CloseSurface(body)
		if err != nil {
			return nil, err
		}
		return marshalBody(response)
	case MethodMoveResizeSurface:
		var body SurfaceLayoutActionRequest
		if err := decodeBody(request.Body, &body); err != nil {
			return nil, err
		}
		response, err := bridge.MoveResizeSurface(body)
		if err != nil {
			return nil, err
		}
		return marshalBody(response)
	case MethodTileSurface:
		var body SurfaceLayoutActionRequest
		if err := decodeBody(request.Body, &body); err != nil {
			return nil, err
		}
		response, err := bridge.TileSurface(body)
		if err != nil {
			return nil, err
		}
		return marshalBody(response)
	case MethodSetSurfaceFloating:
		var body SurfaceLayoutActionRequest
		if err := decodeBody(request.Body, &body); err != nil {
			return nil, err
		}
		response, err := bridge.SetSurfaceFloating(body)
		if err != nil {
			return nil, err
		}
		return marshalBody(response)
	case MethodAssignSurfaceZone:
		var body SurfaceLayoutActionRequest
		if err := decodeBody(request.Body, &body); err != nil {
			return nil, err
		}
		response, err := bridge.AssignSurfaceZone(body)
		if err != nil {
			return nil, err
		}
		return marshalBody(response)
	case MethodPromoteSurface:
		var body SurfaceLayoutActionRequest
		if err := decodeBody(request.Body, &body); err != nil {
			return nil, err
		}
		response, err := bridge.PromoteSurface(body)
		if err != nil {
			return nil, err
		}
		return marshalBody(response)
	case MethodMaximizeSurface:
		var body SurfaceLayoutActionRequest
		if err := decodeBody(request.Body, &body); err != nil {
			return nil, err
		}
		response, err := bridge.MaximizeSurface(body)
		if err != nil {
			return nil, err
		}
		return marshalBody(response)
	case MethodMinimizeSurface:
		var body SurfaceLayoutActionRequest
		if err := decodeBody(request.Body, &body); err != nil {
			return nil, err
		}
		response, err := bridge.MinimizeSurface(body)
		if err != nil {
			return nil, err
		}
		return marshalBody(response)
	case MethodFullscreenSurface:
		var body SurfaceLayoutActionRequest
		if err := decodeBody(request.Body, &body); err != nil {
			return nil, err
		}
		response, err := bridge.FullscreenSurface(body)
		if err != nil {
			return nil, err
		}
		return marshalBody(response)
	case MethodActivateWorkspace:
		var body WorkspaceActionRequest
		if err := decodeBody(request.Body, &body); err != nil {
			return nil, err
		}
		response, err := bridge.ActivateWorkspace(body)
		if err != nil {
			return nil, err
		}
		return marshalBody(response)
	default:
		return nil, classifiedError{class: ErrorBackendUnsupported, message: fmt.Sprintf("unsupported compositor method %q", request.Method)}
	}
}

func (bridge *Bridge) ListSurfaces() []TrackedSurface {
	bridge.mu.Lock()
	defer bridge.mu.Unlock()
	bridge.pruneDeadClientSurfacesLocked(time.Now())
	surfaces := make([]TrackedSurface, 0, len(bridge.surfaces))
	for _, surface := range bridge.surfaces {
		surfaces = append(surfaces, surface)
	}
	sort.Slice(surfaces, func(i, j int) bool { return surfaces[i].Surface.ID < surfaces[j].Surface.ID })
	return surfaces
}

func (bridge *Bridge) pruneDeadClientSurfacesLocked(now time.Time) {
	for id, surface := range bridge.surfaces {
		if surface.Client.PID <= 0 {
			continue
		}
		if now.Sub(surface.UpdatedAt) < deadClientPruneAfter || processExists(int(surface.Client.PID)) {
			continue
		}
		if bridge.promotedSurfaceID == id {
			bridge.promotedSurfaceID = ""
		}
		bridge.stale[id] = now
		delete(bridge.surfaces, id)
		bridge.removeSurfaceFromBackendLayoutLocked(id)
	}
}

func processExists(pid int) bool {
	if pid <= 0 {
		return false
	}
	if err := syscall.Kill(pid, 0); err == nil {
		return true
	} else if errors.Is(err, syscall.EPERM) {
		return true
	}
	return false
}

func (bridge *Bridge) ListOutputs() []LogicalOutput {
	bridge.mu.RLock()
	defer bridge.mu.RUnlock()
	outputs := bridge.outputsLocked()
	names := make([]string, 0, len(outputs))
	for name := range outputs {
		names = append(names, name)
	}
	sort.Strings(names)
	list := make([]LogicalOutput, 0, len(names))
	for _, name := range names {
		list = append(list, outputs[name])
	}
	return list
}

func (bridge *Bridge) GetLayout() GetLayoutResponse {
	bridge.mu.Lock()
	defer bridge.mu.Unlock()
	bridge.pruneDeadClientSurfacesLocked(time.Now())
	return GetLayoutResponse{Layout: bridge.layoutLocked()}
}

func (bridge *Bridge) SetLayoutMode(request SetLayoutModeRequest) (LayoutActionResponse, error) {
	if !validLayoutMode(request.Mode) {
		return LayoutActionResponse{}, fmt.Errorf("unsupported layout mode %q", request.Mode)
	}
	bridge.mu.Lock()
	bridge.layoutMode = request.Mode
	bridge.layoutSettings.Mode = request.Mode
	bridge.layoutSeq++
	for id, tracked := range bridge.surfaces {
		if tracked.Surface.SurfaceKind == SurfaceKindLayer {
			continue
		}
		tracked.LayoutMode = string(request.Mode)
		tracked.Surface.LayoutMode = string(request.Mode)
		tracked.LayoutRevision = bridge.layoutSeq
		bridge.surfaces[id] = tracked
	}
	if bridge.backendLayout != nil {
		layout := cloneLayoutState(*bridge.backendLayout)
		layout.Mode = request.Mode
		layout.Revision = bridge.layoutSeq
		for index := range layout.Surfaces {
			layout.Surfaces[index].Mode = request.Mode
		}
		bridge.backendLayout = cloneLayoutStatePtr(layout)
	}
	layout := bridge.layoutLocked()
	settings := bridge.layoutSettings
	settingsPath := bridge.layoutSettingsPath
	bridge.mu.Unlock()
	if err := SaveLayoutSettings(settingsPath, settings); err != nil {
		return LayoutActionResponse{}, err
	}
	if request.Mode != LayoutModeFreeform {
		bridge.requestAutoLayout("set_layout_mode")
	}
	return LayoutActionResponse{
		Action:   "layout.set_mode",
		Decision: DecisionAccepted,
		Reason:   "layout mode updated",
		Layout:   &layout,
	}, nil
}

func (bridge *Bridge) UpdateLayoutSettings(request UpdateLayoutSettingsRequest) (LayoutActionResponse, error) {
	bridge.mu.Lock()
	settings := bridge.layoutSettings
	if request.Rule != nil {
		settings.Rule = strings.TrimSpace(*request.Rule)
	}
	if request.Mode != nil {
		settings.Mode = *request.Mode
	}
	if request.Gaps != nil {
		settings.Gaps = *request.Gaps
	}
	if request.MasterCount != nil {
		settings.MasterCount = *request.MasterCount
	}
	if request.MasterRatio != nil {
		settings.MasterRatio = *request.MasterRatio
	}
	if request.SmartGaps != nil {
		settings.SmartGaps = *request.SmartGaps
	}
	if err := validateLayoutSettings(settings); err != nil {
		bridge.mu.Unlock()
		return LayoutActionResponse{}, err
	}
	bridge.layoutSettings = settings
	bridge.layoutMode = settings.Mode
	bridge.layoutSeq++
	for id, tracked := range bridge.surfaces {
		if tracked.Surface.SurfaceKind == SurfaceKindLayer {
			continue
		}
		tracked.LayoutMode = string(settings.Mode)
		tracked.Surface.LayoutMode = string(settings.Mode)
		tracked.LayoutRevision = bridge.layoutSeq
		bridge.surfaces[id] = tracked
	}
	if bridge.backendLayout != nil {
		layout := cloneLayoutState(*bridge.backendLayout)
		layout.Mode = settings.Mode
		layout.Settings = settings
		layout.Revision = bridge.layoutSeq
		for index := range layout.Surfaces {
			layout.Surfaces[index].Mode = settings.Mode
		}
		bridge.backendLayout = cloneLayoutStatePtr(layout)
	}
	layout := bridge.layoutLocked()
	settingsPath := bridge.layoutSettingsPath
	bridge.mu.Unlock()
	if err := SaveLayoutSettings(settingsPath, settings); err != nil {
		return LayoutActionResponse{}, err
	}
	if settings.Mode != LayoutModeFreeform {
		bridge.requestAutoLayout("layout_settings")
	}
	return LayoutActionResponse{
		Action:   "layout.update_settings",
		Decision: DecisionAccepted,
		Reason:   "layout settings updated",
		Layout:   &layout,
	}, nil
}

func (bridge *Bridge) MoveResizeSurface(request SurfaceLayoutActionRequest) (LayoutActionResponse, error) {
	if request.Geometry == nil {
		return LayoutActionResponse{}, fmt.Errorf("geometry is required")
	}
	if request.Geometry.Width <= 0 || request.Geometry.Height <= 0 {
		return LayoutActionResponse{}, fmt.Errorf("geometry width and height must be positive")
	}
	return bridge.placeSurface(request, "surface.move_resize", *request.Geometry, request.ZoneID, SurfaceLayoutRoleFloating)
}

func (bridge *Bridge) TileSurface(request SurfaceLayoutActionRequest) (LayoutActionResponse, error) {
	if strings.TrimSpace(request.ZoneID) == "" {
		return LayoutActionResponse{}, fmt.Errorf("zone_id is required")
	}
	geometry, err := bridge.geometryForLayoutRequest(request, "surface.tile")
	if err != nil {
		if class, _ := classifyError(err); class == ErrorBackendUnsupported {
			return bridge.unsupportedSurfaceLayoutAction("surface.tile", request.SurfaceID)
		}
		return LayoutActionResponse{}, err
	}
	return bridge.placeSurface(request, "surface.tile", geometry, request.ZoneID, SurfaceLayoutRoleTiled)
}

func (bridge *Bridge) SetSurfaceFloating(request SurfaceLayoutActionRequest) (LayoutActionResponse, error) {
	if request.Floating == nil {
		return LayoutActionResponse{}, fmt.Errorf("floating is required")
	}
	surface, err := bridge.requireWorkSurface(request.SurfaceID, "surface.set_floating")
	if err != nil {
		return LayoutActionResponse{}, err
	}
	bridge.mu.Lock()
	tracked := bridge.surfaces[request.SurfaceID]
	enabled := *request.Floating
	if request.Enabled != nil {
		enabled = *request.Enabled
	}
	if enabled {
		tracked.LayoutMode = string(LayoutModeFreeform)
		tracked.Surface.LayoutMode = tracked.LayoutMode
		tracked.LayoutRole = string(SurfaceLayoutRoleFloating)
		tracked.Surface.LayoutRole = tracked.LayoutRole
		tracked.ZoneID = zoneTransient
		tracked.Surface.ZoneID = tracked.ZoneID
	} else {
		tracked.LayoutMode = string(bridge.tiledLayoutModeLocked())
		tracked.Surface.LayoutMode = tracked.LayoutMode
		tracked.LayoutRole = string(SurfaceLayoutRoleTiled)
		tracked.Surface.LayoutRole = tracked.LayoutRole
		tracked.ZoneID = firstNonEmpty(tracked.ZoneID, zoneMaster)
		if tracked.ZoneID == zoneTransient {
			tracked.ZoneID = zoneMaster
		}
		tracked.Surface.ZoneID = tracked.ZoneID
	}
	bridge.layoutSeq++
	tracked.LayoutRevision = bridge.layoutSeq
	tracked.UpdatedAt = time.Now()
	bridge.surfaces[request.SurfaceID] = tracked
	bridge.updateBackendLayoutSurfaceLocked(tracked)
	layout := bridge.layoutLocked()
	bridge.mu.Unlock()
	if !enabled {
		bridge.requestAutoLayout("surface_set_tiled")
	} else {
		bridge.requestAutoLayout("surface_set_floating")
	}
	surface = tracked
	return LayoutActionResponse{
		Action:    "surface.set_floating",
		SurfaceID: request.SurfaceID,
		Decision:  DecisionAccepted,
		Reason:    "surface layout participation updated",
		Layout:    &layout,
		Surface:   &surface,
	}, nil
}

func (bridge *Bridge) AssignSurfaceZone(request SurfaceLayoutActionRequest) (LayoutActionResponse, error) {
	if strings.TrimSpace(request.ZoneID) == "" {
		return LayoutActionResponse{}, fmt.Errorf("zone_id is required")
	}
	geometry, err := bridge.geometryForLayoutRequest(request, "surface.assign_zone")
	if err != nil {
		if class, _ := classifyError(err); class == ErrorBackendUnsupported {
			return bridge.unsupportedSurfaceLayoutAction("surface.assign_zone", request.SurfaceID)
		}
		return LayoutActionResponse{}, err
	}
	return bridge.placeSurface(request, "surface.assign_zone", geometry, request.ZoneID, SurfaceLayoutRoleTiled)
}

func (bridge *Bridge) PromoteSurface(request SurfaceLayoutActionRequest) (LayoutActionResponse, error) {
	surface, err := bridge.requireWorkSurface(request.SurfaceID, "surface.promote")
	if err != nil {
		return LayoutActionResponse{}, err
	}
	bridge.mu.Lock()
	bridge.promotedSurfaceID = request.SurfaceID
	targetWorkspaceID := firstNonEmpty(surface.WorkspaceID, surface.Surface.WorkspaceID, bridge.activeWorkspaceIDLocked())
	for id, tracked := range bridge.surfaces {
		if tracked.Surface.SurfaceKind == SurfaceKindLayer {
			continue
		}
		if firstNonEmpty(tracked.WorkspaceID, tracked.Surface.WorkspaceID, bridge.activeWorkspaceIDLocked()) != targetWorkspaceID {
			continue
		}
		tracked.Focused = id == request.SurfaceID
		if id == request.SurfaceID {
			tracked.LayoutMode = string(bridge.tiledLayoutModeLocked())
			tracked.Surface.LayoutMode = tracked.LayoutMode
			tracked.LayoutRole = string(SurfaceLayoutRoleTiled)
			tracked.Surface.LayoutRole = tracked.LayoutRole
			tracked.ZoneID = zoneMaster
			tracked.Surface.ZoneID = tracked.ZoneID
			tracked.LayoutRevision = bridge.layoutSeq + 1
			tracked.UpdatedAt = time.Now()
			surface = tracked
		}
		bridge.surfaces[id] = tracked
	}
	bridge.layoutSeq++
	bridge.updateBackendLayoutFocusLocked(request.SurfaceID)
	layout := bridge.layoutLocked()
	bridge.mu.Unlock()
	bridge.requestAutoLayout("surface_promote")
	return LayoutActionResponse{
		Action:    "surface.promote",
		SurfaceID: request.SurfaceID,
		Decision:  DecisionAccepted,
		Reason:    "surface promoted by layout authority",
		Layout:    &layout,
		Surface:   &surface,
	}, nil
}

func (bridge *Bridge) MaximizeSurface(request SurfaceLayoutActionRequest) (LayoutActionResponse, error) {
	return bridge.setSurfaceState(request, "surface.maximize", "maximized")
}

func (bridge *Bridge) MinimizeSurface(request SurfaceLayoutActionRequest) (LayoutActionResponse, error) {
	return bridge.setSurfaceState(request, "surface.minimize", "minimized")
}

func (bridge *Bridge) FullscreenSurface(request SurfaceLayoutActionRequest) (LayoutActionResponse, error) {
	return bridge.setSurfaceState(request, "surface.fullscreen", "fullscreen")
}

func (bridge *Bridge) ActivateWorkspace(request WorkspaceActionRequest) (LayoutActionResponse, error) {
	workspaceID := strings.TrimSpace(request.WorkspaceID)
	if workspaceID == "" {
		return LayoutActionResponse{}, fmt.Errorf("workspace_id is required")
	}
	type workspaceVisibilityTarget struct {
		SurfaceID string
		Minimized bool
	}
	targets := []workspaceVisibilityTarget{}
	var session *pluginSession
	bridge.mu.Lock()
	bridge.ensureWorkspaceLocked(workspaceID)
	bridge.activeWorkspaceID = workspaceID
	for id, tracked := range bridge.surfaces {
		if tracked.Surface.SurfaceKind == SurfaceKindLayer {
			continue
		}
		surfaceWorkspaceID := firstNonEmpty(tracked.WorkspaceID, tracked.Surface.WorkspaceID, bridge.activeWorkspaceIDLocked())
		tracked.WorkspaceID = surfaceWorkspaceID
		tracked.Surface.WorkspaceID = surfaceWorkspaceID
		active := surfaceWorkspaceID == workspaceID
		tracked.Visible = active
		visible := active
		tracked.Surface.Visible = &visible
		if !active && bridge.promotedSurfaceID == tracked.Surface.ID {
			bridge.promotedSurfaceID = ""
		}
		tracked.LayoutRevision = bridge.layoutSeq + 1
		tracked.UpdatedAt = time.Now()
		bridge.surfaces[id] = tracked
		targets = append(targets, workspaceVisibilityTarget{SurfaceID: tracked.Surface.ID, Minimized: !active})
	}
	bridge.layoutSeq++
	bridge.applyWorkspaceAuthorityToBackendLayoutLocked()
	layout := bridge.layoutLocked()
	session = bridge.plugin
	bridge.mu.Unlock()

	for _, target := range targets {
		if session == nil {
			break
		}
		if err := bridge.sendPluginMessage(session, map[string]any{
			"type":       PluginSetSurfaceState,
			"request_id": fmt.Sprintf("workspace-%d-%s", time.Now().UnixNano(), target.SurfaceID),
			"surface_id": target.SurfaceID,
			"minimized":  target.Minimized,
		}); err != nil {
			break
		}
	}
	bridge.requestAutoLayout("workspace_activate")
	return LayoutActionResponse{
		Action:      "workspace.activate",
		WorkspaceID: workspaceID,
		Decision:    DecisionAccepted,
		Reason:      "workspace activated by bridge authority",
		Layout:      &layout,
	}, nil
}

func (bridge *Bridge) CaptureOutput(request CaptureOutputRequest) (CaptureOutputResponse, error) {
	if request.Name == "" {
		return CaptureOutputResponse{}, fmt.Errorf("output name is required")
	}
	outputs := bridge.ListOutputs()
	var output LogicalOutput
	for _, candidate := range outputs {
		if candidate.Name == request.Name {
			output = candidate
			break
		}
	}
	if output.Name == "" {
		return CaptureOutputResponse{}, classifiedError{class: ErrorSurfaceNotFound, message: fmt.Sprintf("output %s not found", request.Name)}
	}
	capture, err := bridge.capturePhysicalOutput(request, output)
	if err != nil {
		return CaptureOutputResponse{Output: request.Name, Warnings: []string{err.Error()}}, err
	}
	return CaptureOutputResponse{Output: request.Name, Captures: []CaptureSurfaceResponse{capture}}, nil
}

func (bridge *Bridge) unsupportedSurfaceLayoutAction(action string, surfaceID string) (LayoutActionResponse, error) {
	surface, err := bridge.requireWorkSurface(surfaceID, action)
	if err != nil {
		return LayoutActionResponse{}, err
	}
	return LayoutActionResponse{
		Action:    action,
		SurfaceID: surface.Surface.ID,
		Decision:  "unsupported",
		Surface:   &surface,
	}, classifiedError{class: ErrorBackendUnsupported, message: fmt.Sprintf("%s requires compositor backend geometry authority", action)}
}

func (bridge *Bridge) setSurfaceState(request SurfaceLayoutActionRequest, action string, stateField string) (LayoutActionResponse, error) {
	enabled := true
	if request.Enabled != nil {
		enabled = *request.Enabled
	}
	surface, err := bridge.requireStateSurface(request.SurfaceID, action, stateField, enabled)
	if err != nil {
		return LayoutActionResponse{}, err
	}
	session, requestID, waiter, err := bridge.startStateWaiter(request.SurfaceID)
	if err != nil {
		return LayoutActionResponse{}, err
	}
	defer bridge.clearStateWaiter(requestID)

	message := map[string]any{
		"type":       PluginSetSurfaceState,
		"request_id": requestID,
		"surface_id": request.SurfaceID,
		stateField:   enabled,
	}
	if err := bridge.sendPluginMessage(session, message); err != nil {
		return LayoutActionResponse{}, err
	}
	timeout := time.Duration(request.WaitTimeoutMs) * time.Millisecond
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	select {
	case response := <-waiter:
		if !response.OK {
			message := response.Error
			if message == "" {
				message = action + " rejected by compositor plugin"
			}
			class := firstNonEmpty(response.ErrorClass, ErrorProtocol)
			return LayoutActionResponse{}, classifiedError{class: class, message: message}
		}
	case <-time.After(timeout):
		return LayoutActionResponse{}, classifiedError{class: ErrorFrameTimeout, message: action + " request timed out"}
	}

	bridge.mu.Lock()
	if tracked, ok := bridge.surfaces[request.SurfaceID]; ok {
		surface = tracked
	}
	layout := bridge.layoutLocked()
	bridge.mu.Unlock()
	return LayoutActionResponse{
		Action:    action,
		SurfaceID: request.SurfaceID,
		Decision:  DecisionAccepted,
		Reason:    "state changed via compositor plugin",
		Layout:    &layout,
		Surface:   &surface,
	}, nil
}

func (bridge *Bridge) requireStateSurface(surfaceID string, action string, stateField string, enabled bool) (TrackedSurface, error) {
	if surfaceID == "" {
		return TrackedSurface{}, fmt.Errorf("surface_id is required")
	}
	bridge.mu.RLock()
	surface, ok := bridge.surfaces[surfaceID]
	_, stale := bridge.stale[surfaceID]
	bridge.mu.RUnlock()
	if !ok {
		if stale {
			return TrackedSurface{}, classifiedError{class: ErrorSurfaceStale, message: fmt.Sprintf("surface %s is unmapped/stale", surfaceID)}
		}
		return TrackedSurface{}, classifiedError{class: ErrorSurfaceNotFound, message: fmt.Sprintf("surface %s not found", surfaceID)}
	}
	if surface.Surface.SurfaceKind == SurfaceKindLayer {
		return TrackedSurface{}, classifiedError{class: ErrorBackendUnsupported, message: fmt.Sprintf("surface %s is a layer-shell surface and cannot run %s as a work surface", surfaceID, action)}
	}
	if !surface.Visible && !(stateField == "minimized" && !enabled) {
		return TrackedSurface{}, classifiedError{class: ErrorSurfaceStale, message: fmt.Sprintf("surface %s is not visible", surfaceID)}
	}
	return surface, nil
}

func (bridge *Bridge) placeSurface(request SurfaceLayoutActionRequest, action string, geometry SurfaceGeometry, zoneID string, role SurfaceLayoutRole) (LayoutActionResponse, error) {
	return bridge.placeSurfaceChecked(request, action, geometry, zoneID, role, nil, true)
}

func (bridge *Bridge) placeSurfaceChecked(request SurfaceLayoutActionRequest, action string, geometry SurfaceGeometry, zoneID string, role SurfaceLayoutRole, guard func(TrackedSurface) bool, updateBackend bool) (LayoutActionResponse, error) {
	surface, err := bridge.requireWorkSurface(request.SurfaceID, action)
	if err != nil {
		return LayoutActionResponse{}, err
	}
	session, requestID, waiter, err := bridge.startPlaceWaiter(request.SurfaceID)
	if err != nil {
		return LayoutActionResponse{}, err
	}
	defer bridge.clearPlaceWaiter(requestID)

	if err := bridge.sendPluginMessage(session, map[string]any{
		"type":       PluginPlaceSurface,
		"request_id": requestID,
		"surface_id": request.SurfaceID,
		"geometry":   geometry,
	}); err != nil {
		return LayoutActionResponse{}, err
	}
	timeout := time.Duration(request.WaitTimeoutMs) * time.Millisecond
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	ackGeometry := geometry
	select {
	case response := <-waiter:
		if !response.OK {
			message := response.Error
			if message == "" {
				message = "placement rejected by compositor plugin"
			}
			class := firstNonEmpty(response.ErrorClass, ErrorProtocol)
			return LayoutActionResponse{}, classifiedError{class: class, message: message}
		}
		if response.Geometry != nil {
			if response.Geometry.Width <= 0 || response.Geometry.Height <= 0 {
				return LayoutActionResponse{}, classifiedError{class: ErrorProtocol, message: "placement response geometry width and height must be positive"}
			}
			ackGeometry = *response.Geometry
		}
	case <-time.After(timeout):
		return LayoutActionResponse{}, classifiedError{class: ErrorFrameTimeout, message: "placement request timed out"}
	}

	bridge.mu.Lock()
	tracked := bridge.surfaces[request.SurfaceID]
	if guard != nil && !guard(tracked) {
		bridge.mu.Unlock()
		return LayoutActionResponse{}, classifiedError{class: ErrorSurfaceStale, message: fmt.Sprintf("surface %s is no longer eligible for %s", request.SurfaceID, action)}
	}
	tracked.Geometry = cloneGeometry(&ackGeometry)
	tracked.Surface.Geometry = cloneGeometry(&ackGeometry)
	tracked.WorkspaceID = firstNonEmpty(request.WorkspaceID, tracked.WorkspaceID, bridge.activeWorkspaceIDLocked())
	tracked.Surface.WorkspaceID = tracked.WorkspaceID
	tracked.ZoneID = firstNonEmpty(zoneID, tracked.ZoneID, "primary")
	tracked.Surface.ZoneID = tracked.ZoneID
	if role == SurfaceLayoutRoleTiled {
		layoutMode := bridge.tiledLayoutModeLocked()
		tracked.LayoutMode = string(layoutMode)
		tracked.Surface.LayoutMode = string(layoutMode)
		tracked.LayoutRole = string(SurfaceLayoutRoleTiled)
		tracked.Surface.LayoutRole = string(SurfaceLayoutRoleTiled)
	} else {
		tracked.LayoutMode = firstNonEmpty(tracked.LayoutMode, string(LayoutModeFreeform))
		tracked.Surface.LayoutMode = tracked.LayoutMode
		tracked.LayoutRole = firstNonEmpty(tracked.LayoutRole, string(role), string(SurfaceLayoutRoleFloating))
		tracked.Surface.LayoutRole = tracked.LayoutRole
	}
	bridge.layoutSeq++
	tracked.LayoutRevision = bridge.layoutSeq
	tracked.UpdatedAt = time.Now()
	bridge.surfaces[request.SurfaceID] = tracked
	if updateBackend {
		bridge.updateBackendLayoutSurfaceLocked(tracked)
	}
	layout := bridge.layoutLocked()
	bridge.mu.Unlock()

	surface = tracked
	return LayoutActionResponse{
		Action:    action,
		SurfaceID: request.SurfaceID,
		Decision:  DecisionAccepted,
		Reason:    "placed via compositor plugin",
		Layout:    &layout,
		Surface:   &surface,
	}, nil
}

func (bridge *Bridge) geometryForLayoutRequest(request SurfaceLayoutActionRequest, action string) (SurfaceGeometry, error) {
	if request.Geometry != nil {
		if request.Geometry.Width <= 0 || request.Geometry.Height <= 0 {
			return SurfaceGeometry{}, fmt.Errorf("geometry width and height must be positive")
		}
		return *request.Geometry, nil
	}
	return bridge.geometryForZone(request.SurfaceID, request.ZoneID, action)
}

func (bridge *Bridge) geometryForZone(surfaceID string, zoneID string, action string) (SurfaceGeometry, error) {
	surface, err := bridge.requireWorkSurface(surfaceID, action)
	if err != nil {
		return SurfaceGeometry{}, err
	}
	zoneID = strings.TrimSpace(zoneID)
	if zoneID != "primary" && zoneID != "secondary" {
		return SurfaceGeometry{}, classifiedError{class: ErrorBackendUnsupported, message: fmt.Sprintf("zone %s is not supported by the minimal Wayfire layout adapter", zoneID)}
	}

	bridge.mu.RLock()
	defer bridge.mu.RUnlock()
	outputName := firstNonEmpty(surface.OutputID, surface.Surface.OutputID)
	outputs := bridge.outputsLocked()
	var output LogicalOutput
	if outputName != "" {
		output = outputs[outputName]
	}
	if output.Name == "" {
		for _, candidate := range outputs {
			output = candidate
			break
		}
	}
	if output.Name == "" {
		return SurfaceGeometry{}, classifiedError{class: ErrorBackendUnsupported, message: "no output is available for zone placement"}
	}
	width := firstPositive(output.PhysicalWidth, output.Width)
	height := firstPositive(output.PhysicalHeight, output.Height)
	if width <= 0 || height <= 0 {
		return SurfaceGeometry{}, classifiedError{class: ErrorBackendUnsupported, message: "output geometry is not available for zone placement"}
	}
	height -= bridge.reservedBottomHeightLocked(output.Name, width, height)
	if height <= 0 {
		return SurfaceGeometry{}, classifiedError{class: ErrorBackendUnsupported, message: "output work area is empty after chrome reservation"}
	}
	leftWidth := width / 2
	if zoneID == "primary" {
		return SurfaceGeometry{X: output.PhysicalX, Y: output.PhysicalY, Width: leftWidth, Height: height}, nil
	}
	return SurfaceGeometry{X: output.PhysicalX + leftWidth, Y: output.PhysicalY, Width: width - leftWidth, Height: height}, nil
}

func (bridge *Bridge) reservedBottomHeightLocked(outputName string, outputWidth int, outputHeight int) int {
	reserved := 0
	for _, surface := range bridge.surfaces {
		if surface.Surface.SurfaceKind != SurfaceKindLayer || firstNonEmpty(surface.OutputID, surface.Surface.OutputID) != outputName {
			continue
		}
		if firstNonEmpty(surface.Surface.Role, surface.LayoutRole) != "panel" {
			continue
		}
		geometry := firstGeometry(surface)
		if geometry == nil || geometry.Height <= 0 {
			continue
		}
		if outputWidth > 0 && geometry.Width > 0 && geometry.Width < outputWidth/2 {
			continue
		}
		if geometry.Height > reserved {
			reserved = geometry.Height
		}
	}
	if reserved == 0 || bridge.hasLayerShellWorkAreaReadbackLocked(outputName, outputWidth, outputHeight) {
		return 0
	}
	return reserved
}

func (bridge *Bridge) hasLayerShellWorkAreaReadbackLocked(outputName string, outputWidth int, outputHeight int) bool {
	if outputHeight <= 0 {
		return false
	}
	for _, surface := range bridge.surfaces {
		if surface.Surface.SurfaceKind != SurfaceKindLayer || firstNonEmpty(surface.OutputID, surface.Surface.OutputID) != outputName {
			continue
		}
		if firstNonEmpty(surface.Surface.Role, surface.LayoutRole) == "panel" {
			continue
		}
		geometry := firstGeometry(surface)
		if geometry == nil || geometry.Height <= 0 {
			continue
		}
		if outputWidth > 0 && geometry.Width > 0 && geometry.Width < outputWidth/2 {
			continue
		}
		if geometry.Height >= outputHeight {
			return true
		}
	}
	return false
}

func (bridge *Bridge) updateBackendLayoutSurfaceLocked(tracked TrackedSurface) {
	if bridge.backendLayout == nil {
		return
	}
	layout := cloneLayoutState(*bridge.backendLayout)
	found := false
	for index := range layout.Surfaces {
		if layout.Surfaces[index].SurfaceID != tracked.Surface.ID {
			continue
		}
		layout.Surfaces[index].Geometry = cloneGeometry(tracked.Geometry)
		layout.Surfaces[index].WorkspaceID = tracked.WorkspaceID
		layout.Surfaces[index].ZoneID = tracked.ZoneID
		layout.Surfaces[index].Mode = LayoutMode(tracked.LayoutMode)
		layout.Surfaces[index].Participation = SurfaceLayoutRole(tracked.LayoutRole)
		layout.Surfaces[index].Floating = tracked.LayoutRole == string(SurfaceLayoutRoleFloating)
		found = true
		break
	}
	if !found {
		layout.Surfaces = append(layout.Surfaces, LayoutSurface{
			SurfaceID:     tracked.Surface.ID,
			Label:         tracked.Surface.Label,
			AppID:         tracked.Surface.AppID,
			Title:         tracked.Surface.Title,
			Role:          tracked.Surface.Role,
			OutputID:      tracked.OutputID,
			WorkspaceID:   tracked.WorkspaceID,
			ZoneID:        tracked.ZoneID,
			Mode:          LayoutMode(tracked.LayoutMode),
			Participation: SurfaceLayoutRole(tracked.LayoutRole),
			Floating:      tracked.LayoutRole == string(SurfaceLayoutRoleFloating),
			Focused:       tracked.Focused,
			Visible:       tracked.Visible,
			Geometry:      cloneGeometry(tracked.Geometry),
			Order:         len(layout.Surfaces),
		})
	}
	layout.Revision = bridge.layoutSeq
	bridge.applyWorkspaceAuthorityToLayoutLocked(&layout)
	bridge.backendLayout = cloneLayoutStatePtr(layout)
}

func (bridge *Bridge) updateBackendLayoutFocusLocked(surfaceID string) {
	if bridge.backendLayout == nil {
		return
	}
	layout := cloneLayoutState(*bridge.backendLayout)
	for index := range layout.Surfaces {
		if tracked, ok := bridge.surfaces[layout.Surfaces[index].SurfaceID]; ok {
			layout.Surfaces[index].Focused = bridge.layoutFocusedLocked(tracked)
			continue
		}
		layout.Surfaces[index].Focused = bridge.promotedSurfaceID == "" && layout.Surfaces[index].SurfaceID == surfaceID
	}
	layout.Revision = bridge.layoutSeq
	bridge.backendLayout = cloneLayoutStatePtr(layout)
}

func (bridge *Bridge) preserveTrackedFocusLocked(layout *LayoutState) {
	for index := range layout.Surfaces {
		tracked, ok := bridge.surfaces[layout.Surfaces[index].SurfaceID]
		if !ok {
			continue
		}
		layout.Surfaces[index].Focused = bridge.layoutFocusedLocked(tracked)
	}
}

func (bridge *Bridge) layoutFocusedLocked(tracked TrackedSurface) bool {
	if bridge.promotedSurfaceID != "" {
		return tracked.Surface.ID == bridge.promotedSurfaceID
	}
	return tracked.Focused
}

func firstPositive(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func minInt(left int, right int) int {
	if left < right {
		return left
	}
	return right
}

func maxInt(left int, right int) int {
	if left > right {
		return left
	}
	return right
}

func (bridge *Bridge) requireWorkSurface(surfaceID string, action string) (TrackedSurface, error) {
	if surfaceID == "" {
		return TrackedSurface{}, fmt.Errorf("surface_id is required")
	}
	bridge.mu.RLock()
	surface, ok := bridge.surfaces[surfaceID]
	_, stale := bridge.stale[surfaceID]
	bridge.mu.RUnlock()
	if !ok {
		if stale {
			return TrackedSurface{}, classifiedError{class: ErrorSurfaceStale, message: fmt.Sprintf("surface %s is unmapped/stale", surfaceID)}
		}
		return TrackedSurface{}, classifiedError{class: ErrorSurfaceNotFound, message: fmt.Sprintf("surface %s not found", surfaceID)}
	}
	if surface.Surface.SurfaceKind == SurfaceKindLayer {
		return TrackedSurface{}, classifiedError{class: ErrorBackendUnsupported, message: fmt.Sprintf("surface %s is a layer-shell surface and cannot run %s as a work surface", surfaceID, action)}
	}
	if !surface.Visible {
		return TrackedSurface{}, classifiedError{class: ErrorSurfaceStale, message: fmt.Sprintf("surface %s is not visible", surfaceID)}
	}
	return surface, nil
}

func (bridge *Bridge) FocusSurface(request FocusSurfaceRequest) (SurfaceActionResponse, error) {
	if request.SurfaceID == "" {
		return SurfaceActionResponse{}, fmt.Errorf("surface_id is required")
	}
	bridge.mu.RLock()
	surface, ok := bridge.surfaces[request.SurfaceID]
	_, stale := bridge.stale[request.SurfaceID]
	bridge.mu.RUnlock()
	if !ok {
		if stale {
			return SurfaceActionResponse{}, classifiedError{class: ErrorSurfaceStale, message: fmt.Sprintf("surface %s is unmapped/stale", request.SurfaceID)}
		}
		return SurfaceActionResponse{}, classifiedError{class: ErrorSurfaceNotFound, message: fmt.Sprintf("surface %s not found", request.SurfaceID)}
	}
	if surface.Surface.SurfaceKind == SurfaceKindLayer {
		return SurfaceActionResponse{}, classifiedError{class: ErrorBackendUnsupported, message: fmt.Sprintf("surface %s is a layer-shell surface and cannot be focused as a work surface", request.SurfaceID)}
	}
	if !surface.Visible {
		return SurfaceActionResponse{}, classifiedError{class: ErrorSurfaceStale, message: fmt.Sprintf("surface %s is not visible", request.SurfaceID)}
	}

	session, requestID, waiter, err := bridge.startFocusWaiter(request.SurfaceID)
	if err != nil {
		return SurfaceActionResponse{}, err
	}
	defer bridge.clearFocusWaiter(requestID)

	if err := bridge.sendPluginMessage(session, map[string]any{"type": PluginFocusSurface, "request_id": requestID, "surface_id": request.SurfaceID}); err != nil {
		return SurfaceActionResponse{}, err
	}
	timeout := time.Duration(request.WaitTimeoutMs) * time.Millisecond
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	select {
	case response := <-waiter:
		if !response.OK {
			message := response.Error
			if message == "" {
				message = "focus rejected by compositor plugin"
			}
			class := firstNonEmpty(response.ErrorClass, ErrorProtocol)
			return SurfaceActionResponse{}, classifiedError{class: class, message: message}
		}
	case <-time.After(timeout):
		return SurfaceActionResponse{}, classifiedError{class: ErrorFrameTimeout, message: "focus request timed out"}
	}

	bridge.mu.Lock()
	targetWorkspaceID := firstNonEmpty(surface.WorkspaceID, surface.Surface.WorkspaceID, bridge.activeWorkspaceIDLocked())
	for id, tracked := range bridge.surfaces {
		if tracked.Surface.SurfaceKind == SurfaceKindLayer {
			continue
		}
		if firstNonEmpty(tracked.WorkspaceID, tracked.Surface.WorkspaceID, bridge.activeWorkspaceIDLocked()) != targetWorkspaceID {
			continue
		}
		tracked.Focused = id == request.SurfaceID
		if id == request.SurfaceID {
			tracked.LayoutRevision = bridge.layoutSeq + 1
			tracked.UpdatedAt = time.Now()
			surface = tracked
		}
		bridge.surfaces[id] = tracked
	}
	bridge.promotedSurfaceID = request.SurfaceID
	bridge.layoutSeq++
	bridge.updateBackendLayoutFocusLocked(request.SurfaceID)
	bridge.mu.Unlock()
	bridge.requestAutoLayout("surface_focus_command")

	return SurfaceActionResponse{
		Action:           "surface.focus",
		SurfaceID:        request.SurfaceID,
		Decision:         DecisionAccepted,
		Reason:           "focused via compositor plugin",
		FocusedSurfaceID: request.SurfaceID,
		Surface:          &surface,
	}, nil
}

func (bridge *Bridge) CloseSurface(request CloseSurfaceRequest) (SurfaceActionResponse, error) {
	if request.SurfaceID == "" {
		return SurfaceActionResponse{}, fmt.Errorf("surface_id is required")
	}
	bridge.mu.RLock()
	surface, ok := bridge.surfaces[request.SurfaceID]
	_, stale := bridge.stale[request.SurfaceID]
	session := bridge.plugin
	bridge.mu.RUnlock()
	if !ok {
		if stale {
			return SurfaceActionResponse{}, classifiedError{class: ErrorSurfaceStale, message: fmt.Sprintf("surface %s is unmapped/stale", request.SurfaceID)}
		}
		return SurfaceActionResponse{}, classifiedError{class: ErrorSurfaceNotFound, message: fmt.Sprintf("surface %s not found", request.SurfaceID)}
	}
	if surface.Surface.SurfaceKind == SurfaceKindLayer {
		return SurfaceActionResponse{}, classifiedError{class: ErrorBackendUnsupported, message: fmt.Sprintf("surface %s is a layer-shell surface and cannot be closed as a work surface", request.SurfaceID)}
	}
	if !surface.Visible {
		return SurfaceActionResponse{}, classifiedError{class: ErrorSurfaceStale, message: fmt.Sprintf("surface %s is not visible", request.SurfaceID)}
	}
	if session == nil {
		return SurfaceActionResponse{}, classifiedError{class: ErrorCompositorUnavailable, message: "no plugin connected"}
	}
	if err := bridge.sendPluginMessage(session, map[string]any{"type": PluginCloseSurface, "surface_id": request.SurfaceID}); err != nil {
		return SurfaceActionResponse{}, err
	}
	return SurfaceActionResponse{
		Action:          "surface.close",
		SurfaceID:       request.SurfaceID,
		Decision:        DecisionAccepted,
		Reason:          "close queued via compositor plugin",
		ClosedSurfaceID: request.SurfaceID,
		Queued:          true,
		Surface:         &surface,
	}, nil
}

func (bridge *Bridge) handlePluginEvent(event pluginEvent) {
	switch event.Type {
	case PluginSurfaceEvent:
		bridge.handleSurfaceEvent(event)
	case PluginLayoutState:
		bridge.handleLayoutState(event.Layout)
	case PluginFocusResponse:
		bridge.handleFocusResponse(event)
	case PluginPlaceResponse:
		bridge.handlePlaceResponse(event)
	case PluginSurfaceStateResponse:
		bridge.handleSurfaceStateResponse(event)
	}
}

func (bridge *Bridge) handleLayoutState(layout LayoutState) {
	if !validLayoutMode(layout.Mode) {
		layout.Mode = LayoutModeZones
	}
	if layout.Revision == 0 {
		bridge.mu.Lock()
		bridge.layoutSeq++
		layout.Revision = bridge.layoutSeq
		bridge.mu.Unlock()
	} else {
		bridge.mu.Lock()
		if layout.Revision > bridge.layoutSeq {
			bridge.layoutSeq = layout.Revision
		}
		bridge.mu.Unlock()
	}
	bridge.mu.Lock()
	defer bridge.mu.Unlock()
	for index := range layout.Surfaces {
		if layout.Surfaces[index].WorkspaceID == "" {
			layout.Surfaces[index].WorkspaceID = bridge.activeWorkspaceIDLocked()
		}
		bridge.ensureWorkspaceLocked(layout.Surfaces[index].WorkspaceID)
	}
	for _, workspace := range layout.Workspaces {
		bridge.ensureWorkspaceLocked(workspace.ID)
	}
	bridge.applyWorkspaceAuthorityToLayoutLocked(&layout)
	bridge.layoutMode = layout.Mode
	layout.Settings = bridge.layoutSettings
	bridge.preserveTrackedFocusLocked(&layout)
	bridge.backendLayout = cloneLayoutStatePtr(layout)
	for _, layoutSurface := range layout.Surfaces {
		if layoutSurface.SurfaceID == "" {
			continue
		}
		tracked := bridge.surfaces[layoutSurface.SurfaceID]
		previousFocused := tracked.Focused
		visible := layoutSurface.Visible
		tracked.Surface.ID = layoutSurface.SurfaceID
		tracked.Surface.Label = layoutSurface.Label
		tracked.Surface.AppID = firstNonEmpty(layoutSurface.AppID, tracked.Surface.AppID)
		tracked.Surface.Title = firstNonEmpty(layoutSurface.Title, tracked.Surface.Title)
		tracked.Surface.Role = firstNonEmpty(layoutSurface.Role, tracked.Surface.Role, "toplevel")
		tracked.Surface.SurfaceKind = firstNonEmpty(tracked.Surface.SurfaceKind, SurfaceKindXDG)
		tracked.Surface.OutputID = layoutSurface.OutputID
		tracked.Surface.WorkspaceID = layoutSurface.WorkspaceID
		tracked.Surface.ZoneID = layoutSurface.ZoneID
		tracked.Surface.LayoutMode = string(layoutSurface.Mode)
		tracked.Surface.LayoutRole = string(layoutSurface.Participation)
		tracked.Surface.Geometry = cloneGeometry(layoutSurface.Geometry)
		tracked.Surface.Visible = &visible
		tracked.Geometry = cloneGeometry(layoutSurface.Geometry)
		tracked.OutputID = layoutSurface.OutputID
		tracked.WorkspaceID = layoutSurface.WorkspaceID
		tracked.ZoneID = layoutSurface.ZoneID
		tracked.LayoutMode = string(layoutSurface.Mode)
		tracked.LayoutRole = string(layoutSurface.Participation)
		tracked.LayoutRevision = layout.Revision
		tracked.Focused = previousFocused
		tracked.Visible = layoutSurface.Visible
		tracked.Capturable = tracked.Surface.SurfaceKind != SurfaceKindLayer
		tracked.InputInjectable = tracked.Surface.SurfaceKind != SurfaceKindLayer
		tracked.LastEvent = firstNonEmpty(tracked.LastEvent, EventMapped)
		tracked.UpdatedAt = time.Now()
		if tracked.ScaleFactor == 0 {
			tracked.ScaleFactor = 1
		}
		if tracked.Surface.ScaleFactor == 0 {
			tracked.Surface.ScaleFactor = tracked.ScaleFactor
		}
		bridge.surfaces[layoutSurface.SurfaceID] = tracked
		delete(bridge.stale, layoutSurface.SurfaceID)
	}
}

func (bridge *Bridge) handleSurfaceEvent(event pluginEvent) {
	if event.Surface.ID == "" {
		return
	}
	now := time.Now()
	if event.Surface.SurfaceKind == "" {
		event.Surface.SurfaceKind = SurfaceKindXDG
	}
	visible := true
	if event.Surface.Visible != nil {
		visible = *event.Surface.Visible
	}
	tracked := TrackedSurface{
		Surface:         event.Surface,
		Client:          event.Client,
		LastEvent:       event.Event,
		Device:          event.Device,
		UpdatedAt:       now,
		Geometry:        event.Surface.Geometry,
		PixelSize:       event.Surface.PixelSize,
		ScaleFactor:     event.Surface.ScaleFactor,
		Capturable:      event.Surface.SurfaceKind != SurfaceKindLayer,
		InputInjectable: event.Surface.SurfaceKind != SurfaceKindLayer,
		Visible:         visible,
		OutputID:        event.Surface.OutputID,
	}
	if tracked.ScaleFactor == 0 {
		tracked.ScaleFactor = 1
	}
	tracked.WorkspaceID = event.Surface.WorkspaceID
	tracked.ZoneID = event.Surface.ZoneID
	tracked.LayoutMode = event.Surface.LayoutMode
	tracked.LayoutRole = event.Surface.LayoutRole
	tracked.Surface.WorkspaceID = tracked.WorkspaceID
	tracked.Surface.ZoneID = tracked.ZoneID
	tracked.Surface.LayoutMode = tracked.LayoutMode
	tracked.Surface.LayoutRole = tracked.LayoutRole
	if event.Event == EventFocused {
		tracked.Focused = true
	}

	shouldRelayout := false
	bridge.mu.Lock()
	if event.Event == EventUnmapped {
		if bridge.promotedSurfaceID == event.Surface.ID {
			bridge.promotedSurfaceID = ""
		}
		bridge.stale[event.Surface.ID] = now
		delete(bridge.surfaces, event.Surface.ID)
		bridge.removeSurfaceFromBackendLayoutLocked(event.Surface.ID)
		shouldRelayout = true
		bridge.mu.Unlock()
		bridge.requestAutoLayout("surface_unmapped")
		return
	}
	if event.Event == EventFocused {
		for id, other := range bridge.surfaces {
			if id != event.Surface.ID {
				other.Focused = false
				bridge.surfaces[id] = other
			}
		}
	}
	if previous, ok := bridge.surfaces[event.Surface.ID]; ok {
		mergeSurfaceReadback(&tracked, previous, event.Event, now)
	}
	if tracked.WorkspaceID == "" {
		tracked.WorkspaceID = bridge.activeWorkspaceIDLocked()
		tracked.Surface.WorkspaceID = tracked.WorkspaceID
	}
	bridge.ensureWorkspaceLocked(tracked.WorkspaceID)
	applyLayoutDefaults(&tracked)
	bridge.applyLifecycleClassificationLocked(&tracked)
	if event.Event == EventFrameDone {
		tracked.FrameCount++
		tracked.LastPresentTimestamp = &now
	}
	if event.Event == EventContentCommit {
		tracked.ContentCommitCount++
		tracked.LastContentCommitTimestamp = &now
	}
	bridge.layoutSeq++
	tracked.LayoutRevision = bridge.layoutSeq
	delete(bridge.stale, event.Surface.ID)
	bridge.surfaces[event.Surface.ID] = tracked
	if event.Event == EventFocused {
		bridge.updateBackendLayoutFocusLocked(event.Surface.ID)
	}
	switch event.Event {
	case EventMapped:
		shouldRelayout = true
	}
	bridge.mu.Unlock()
	if shouldRelayout {
		bridge.requestAutoLayout("surface_" + event.Event)
	}
}

func mergeSurfaceReadback(next *TrackedSurface, previous TrackedSurface, event string, now time.Time) {
	if next.Surface.AppID == "" {
		next.Surface.AppID = previous.Surface.AppID
	}
	if next.Surface.Title == "" {
		next.Surface.Title = previous.Surface.Title
	}
	if next.Surface.Role == "" {
		next.Surface.Role = previous.Surface.Role
	}
	if next.Surface.Label == "" {
		next.Surface.Label = previous.Surface.Label
	}
	if next.Geometry == nil {
		next.Geometry = previous.Geometry
		next.Surface.Geometry = previous.Surface.Geometry
	}
	if next.PixelSize == nil {
		next.PixelSize = previous.PixelSize
		next.Surface.PixelSize = previous.Surface.PixelSize
	}
	if next.OutputID == "" {
		next.OutputID = previous.OutputID
		next.Surface.OutputID = previous.OutputID
	}
	if next.WorkspaceID == "" {
		next.WorkspaceID = previous.WorkspaceID
		next.Surface.WorkspaceID = previous.WorkspaceID
	}
	if next.ZoneID == "" {
		next.ZoneID = previous.ZoneID
		next.Surface.ZoneID = previous.ZoneID
	}
	if next.LayoutMode == "" {
		next.LayoutMode = previous.LayoutMode
		next.Surface.LayoutMode = previous.LayoutMode
	}
	if next.LayoutRole == "" {
		next.LayoutRole = previous.LayoutRole
		next.Surface.LayoutRole = previous.LayoutRole
	}
	next.FrameCount = previous.FrameCount
	next.LastPresentTimestamp = previous.LastPresentTimestamp
	next.ContentCommitCount = previous.ContentCommitCount
	next.LastContentCommitTimestamp = previous.LastContentCommitTimestamp
	next.CaptureCount = previous.CaptureCount
	next.LastCaptureTimestamp = previous.LastCaptureTimestamp
	if event != EventFocused {
		next.Focused = previous.Focused
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func applyLayoutDefaults(surface *TrackedSurface) {
	surface.WorkspaceID = firstNonEmpty(surface.WorkspaceID, defaultWorkspaceID)
	surface.LayoutMode = firstNonEmpty(surface.LayoutMode, string(LayoutModeFreeform))
	if surface.Surface.SurfaceKind == SurfaceKindLayer {
		surface.ZoneID = firstNonEmpty(surface.ZoneID, "chrome")
		surface.LayoutRole = firstNonEmpty(surface.LayoutRole, string(SurfaceLayoutRoleTransient))
	} else {
		surface.ZoneID = firstNonEmpty(surface.ZoneID, "primary")
		surface.LayoutRole = firstNonEmpty(surface.LayoutRole, string(SurfaceLayoutRoleFloating))
	}
	surface.Surface.WorkspaceID = surface.WorkspaceID
	surface.Surface.ZoneID = surface.ZoneID
	surface.Surface.LayoutMode = surface.LayoutMode
	surface.Surface.LayoutRole = surface.LayoutRole
}

func (bridge *Bridge) handleFocusResponse(event pluginEvent) {
	bridge.mu.RLock()
	waiter := bridge.focusWaiters[event.RequestID]
	bridge.mu.RUnlock()
	if waiter == nil {
		return
	}
	select {
	case waiter <- pluginResponse{OK: event.OK, Error: event.Error}:
	default:
	}
}

func (bridge *Bridge) handlePlaceResponse(event pluginEvent) {
	bridge.mu.RLock()
	waiter := bridge.placeWaiters[event.RequestID]
	bridge.mu.RUnlock()
	if waiter == nil {
		return
	}
	select {
	case waiter <- pluginResponse{OK: event.OK, Error: event.Error, Geometry: cloneGeometry(event.Geometry)}:
	default:
	}
}

func (bridge *Bridge) handleSurfaceStateResponse(event pluginEvent) {
	bridge.mu.RLock()
	waiter := bridge.stateWaiters[event.RequestID]
	bridge.mu.RUnlock()
	if waiter == nil {
		return
	}
	select {
	case waiter <- pluginResponse{OK: event.OK, Error: event.Error}:
	default:
	}
}

func (bridge *Bridge) outputsLocked() map[string]LogicalOutput {
	outputs := map[string]LogicalOutput{}
	stableWidths := map[string]int{}
	stableWorkHeights := map[string]int{}
	for _, surface := range bridge.surfaces {
		name := surface.OutputID
		if name == "" {
			name = surface.Surface.OutputID
		}
		if name == "" {
			continue
		}
		geom := surface.Geometry
		if geom == nil {
			geom = surface.Surface.Geometry
		}
		out := outputs[name]
		if out.Name == "" {
			now := time.Now()
			out = LogicalOutput{Name: name, Scale: 1, Mode: "physical_surface_readback", CreatedAt: now, UpdatedAt: now}
		}
		out.Surfaces = append(out.Surfaces, surface.Surface.ID)
		if geom != nil {
			width := geom.X + geom.Width
			height := geom.Y + geom.Height
			if stableLayerOutputWidth(surface) && geom.Width > stableWidths[name] {
				stableWidths[name] = geom.Width
			}
			if stableLayerWorkAreaHeight(surface) && geom.Height > stableWorkHeights[name] {
				stableWorkHeights[name] = geom.Height
			}
			if geom.Width > out.Width {
				out.Width = geom.Width
			}
			if geom.Height > out.Height {
				out.Height = geom.Height
			}
			if width > out.PhysicalWidth {
				out.PhysicalWidth = width
			}
			if height > out.PhysicalHeight {
				out.PhysicalHeight = height
			}
		}
		if out.Width == 0 {
			out.Width = 1920
		}
		if out.Height == 0 {
			out.Height = 1080
		}
		if out.PhysicalWidth == 0 {
			out.PhysicalWidth = out.Width
		}
		if out.PhysicalHeight == 0 {
			out.PhysicalHeight = out.Height
		}
		outputs[name] = out
	}
	for name, output := range outputs {
		if width := stableWidths[name]; width > 0 {
			output.Width = width
			output.PhysicalWidth = width
		}
		if height := stableWorkHeights[name]; height > 0 {
			output.Height = height
			output.PhysicalHeight = height
		}
		sort.Strings(output.Surfaces)
		outputs[name] = output
	}
	return outputs
}

func stableLayerOutputWidth(surface TrackedSurface) bool {
	if surface.Surface.SurfaceKind != SurfaceKindLayer {
		return false
	}
	switch firstNonEmpty(surface.Surface.Role, surface.LayoutRole, layerShellRole(surface)) {
	case "panel", "background":
		return true
	default:
		return false
	}
}

func stableLayerWorkAreaHeight(surface TrackedSurface) bool {
	if surface.Surface.SurfaceKind != SurfaceKindLayer {
		return false
	}
	return firstNonEmpty(surface.Surface.Role, surface.LayoutRole, layerShellRole(surface)) == "background"
}

func layerShellRole(surface TrackedSurface) string {
	if surface.Surface.LayerShell == nil {
		return ""
	}
	return firstNonEmpty(surface.Surface.LayerShell.EffectiveRole, surface.Surface.LayerShell.HelperRole)
}

func (bridge *Bridge) layoutLocked() LayoutState {
	if bridge.backendLayout != nil {
		layout := cloneLayoutState(*bridge.backendLayout)
		bridge.applyWorkspaceAuthorityToLayoutLocked(&layout)
		return layout
	}
	return bridge.layoutFromTrackedLocked()
}

func (bridge *Bridge) layoutFromTrackedLocked() LayoutState {
	surfaces := make([]TrackedSurface, 0, len(bridge.surfaces))
	for _, surface := range bridge.surfaces {
		if surface.Surface.SurfaceKind == SurfaceKindLayer {
			continue
		}
		bridge.ensureWorkspaceLocked(firstNonEmpty(surface.WorkspaceID, surface.Surface.WorkspaceID, bridge.activeWorkspaceIDLocked()))
		surfaces = append(surfaces, surface)
	}
	workspaceRank := bridge.workspaceRankLocked()
	sort.Slice(surfaces, func(i, j int) bool {
		leftWorkspace := firstNonEmpty(surfaces[i].WorkspaceID, surfaces[i].Surface.WorkspaceID, bridge.activeWorkspaceIDLocked())
		rightWorkspace := firstNonEmpty(surfaces[j].WorkspaceID, surfaces[j].Surface.WorkspaceID, bridge.activeWorkspaceIDLocked())
		if leftWorkspace != rightWorkspace {
			leftRank, leftOK := workspaceRank[leftWorkspace]
			rightRank, rightOK := workspaceRank[rightWorkspace]
			if leftOK && rightOK {
				return leftRank < rightRank
			}
			if leftOK != rightOK {
				return leftOK
			}
			return leftWorkspace < rightWorkspace
		}
		left := firstNonEmpty(surfaces[i].Surface.Label, surfaces[i].Surface.ID)
		right := firstNonEmpty(surfaces[j].Surface.Label, surfaces[j].Surface.ID)
		if left == right {
			return surfaces[i].Surface.ID < surfaces[j].Surface.ID
		}
		return left < right
	})

	builders := map[string]*layoutWorkspaceBuilder{}
	workspaceOrder := bridge.workspaceOrderLocked()
	for _, workspaceID := range workspaceOrder {
		record := bridge.ensureWorkspaceLocked(workspaceID)
		builders[workspaceID] = newWorkspaceBuilder(record, bridge.activeWorkspaceIDLocked())
	}
	layoutSurfaces := make([]LayoutSurface, 0, len(surfaces))
	mode := bridge.layoutMode
	if mode == "" || !validLayoutMode(mode) {
		mode = LayoutModeZones
	}
	activeWorkspaceID := bridge.activeWorkspaceIDLocked()
	for index, surface := range surfaces {
		workspaceID := firstNonEmpty(surface.WorkspaceID, surface.Surface.WorkspaceID, activeWorkspaceID)
		record := bridge.ensureWorkspaceLocked(workspaceID)
		builder := builders[workspaceID]
		if builder == nil {
			builder = newWorkspaceBuilder(record, activeWorkspaceID)
			builders[workspaceID] = builder
			workspaceOrder = append(workspaceOrder, workspaceID)
		}
		zoneID := firstNonEmpty(surface.ZoneID, "primary")
		if _, ok := builder.zones[zoneID]; !ok {
			builder.zones[zoneID] = &LayoutZone{ID: zoneID, Name: zoneID, Kind: zoneKindForSurface(surface)}
			builder.zoneOrder = append(builder.zoneOrder, zoneID)
		}
		builder.zones[zoneID].SurfaceIDs = append(builder.zones[zoneID].SurfaceIDs, surface.Surface.ID)
		builder.workspace.SurfaceOrder = append(builder.workspace.SurfaceOrder, surface.Surface.ID)
		if builder.workspace.OutputID == "" {
			builder.workspace.OutputID = firstNonEmpty(surface.OutputID, surface.Surface.OutputID)
		}
		if surface.LayoutMode != "" && validLayoutMode(LayoutMode(surface.LayoutMode)) {
			mode = LayoutMode(surface.LayoutMode)
		}
		role := SurfaceLayoutRole(surface.LayoutRole)
		if role == "" {
			role = SurfaceLayoutRoleFloating
		}
		label := surface.Surface.Label
		if label == "" {
			label = fmt.Sprintf("%d", index+1)
		}
		active := workspaceID == activeWorkspaceID
		layoutSurfaces = append(layoutSurfaces, LayoutSurface{
			SurfaceID:     surface.Surface.ID,
			Label:         label,
			AppID:         surface.Surface.AppID,
			Title:         surface.Surface.Title,
			Role:          surface.Surface.Role,
			OutputID:      firstNonEmpty(surface.OutputID, surface.Surface.OutputID),
			WorkspaceID:   workspaceID,
			ZoneID:        zoneID,
			Mode:          mode,
			Participation: role,
			Floating:      role == SurfaceLayoutRoleFloating,
			Focused:       active && bridge.layoutFocusedLocked(surface),
			Visible:       active && surface.Visible,
			Geometry:      firstGeometry(surface),
			Order:         index,
		})
	}

	workspaces := make([]LayoutWorkspace, 0, len(workspaceOrder))
	for _, workspaceID := range workspaceOrder {
		builder := builders[workspaceID]
		if builder == nil {
			continue
		}
		for _, zoneID := range builder.zoneOrder {
			builder.workspace.Zones = append(builder.workspace.Zones, *builder.zones[zoneID])
		}
		workspaces = append(workspaces, builder.workspace)
	}
	return LayoutState{
		Mode:       mode,
		Revision:   bridge.layoutSeq,
		Settings:   bridge.layoutSettings,
		Surfaces:   layoutSurfaces,
		Workspaces: workspaces,
	}
}

func (bridge *Bridge) removeSurfaceFromBackendLayoutLocked(surfaceID string) {
	if bridge.backendLayout == nil {
		return
	}
	layout := cloneLayoutState(*bridge.backendLayout)
	filtered := layout.Surfaces[:0]
	for _, surface := range layout.Surfaces {
		if surface.SurfaceID != surfaceID {
			filtered = append(filtered, surface)
		}
	}
	layout.Surfaces = filtered
	for index := range layout.Workspaces {
		workspace := &layout.Workspaces[index]
		workspace.SurfaceOrder = filterString(workspace.SurfaceOrder, surfaceID)
		for zoneIndex := range workspace.Zones {
			workspace.Zones[zoneIndex].SurfaceIDs = filterString(workspace.Zones[zoneIndex].SurfaceIDs, surfaceID)
		}
	}
	bridge.layoutSeq++
	layout.Revision = bridge.layoutSeq
	bridge.applyWorkspaceAuthorityToLayoutLocked(&layout)
	bridge.backendLayout = cloneLayoutStatePtr(layout)
}

func filterString(values []string, remove string) []string {
	filtered := values[:0]
	for _, value := range values {
		if value != remove {
			filtered = append(filtered, value)
		}
	}
	return filtered
}

func (bridge *Bridge) activeWorkspaceIDLocked() string {
	if strings.TrimSpace(bridge.activeWorkspaceID) == "" {
		bridge.activeWorkspaceID = defaultWorkspaceID
	}
	bridge.ensureWorkspaceLocked(bridge.activeWorkspaceID)
	return bridge.activeWorkspaceID
}

func (bridge *Bridge) ensureWorkspaceLocked(workspaceID string) workspaceRecord {
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		workspaceID = defaultWorkspaceID
	}
	if bridge.workspaces == nil {
		bridge.workspaces = map[string]workspaceRecord{}
	}
	if record, ok := bridge.workspaces[workspaceID]; ok {
		if record.Name == "" {
			record.Name = workspaceDisplayName(workspaceID)
			bridge.workspaces[workspaceID] = record
		}
		return record
	}
	record := workspaceRecord{ID: workspaceID, Name: workspaceDisplayName(workspaceID)}
	bridge.workspaces[workspaceID] = record
	bridge.workspaceOrder = append(bridge.workspaceOrder, workspaceID)
	return record
}

func (bridge *Bridge) workspaceOrderLocked() []string {
	bridge.ensureWorkspaceLocked(defaultWorkspaceID)
	bridge.ensureWorkspaceLocked(bridge.activeWorkspaceIDLocked())
	seen := map[string]bool{}
	ordered := make([]string, 0, len(bridge.workspaceOrder)+len(bridge.workspaces))
	for _, workspaceID := range bridge.workspaceOrder {
		if workspaceID == "" || seen[workspaceID] {
			continue
		}
		seen[workspaceID] = true
		ordered = append(ordered, workspaceID)
	}
	extras := make([]string, 0, len(bridge.workspaces))
	for workspaceID := range bridge.workspaces {
		if !seen[workspaceID] {
			extras = append(extras, workspaceID)
		}
	}
	sort.Strings(extras)
	ordered = append(ordered, extras...)
	bridge.workspaceOrder = ordered
	return append([]string(nil), ordered...)
}

func (bridge *Bridge) workspaceRankLocked() map[string]int {
	order := bridge.workspaceOrderLocked()
	rank := make(map[string]int, len(order))
	for index, workspaceID := range order {
		rank[workspaceID] = index
	}
	return rank
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

type layoutWorkspaceBuilder struct {
	workspace LayoutWorkspace
	zones     map[string]*LayoutZone
	zoneOrder []string
}

func newWorkspaceBuilder(record workspaceRecord, activeWorkspaceID string) *layoutWorkspaceBuilder {
	return &layoutWorkspaceBuilder{
		workspace: LayoutWorkspace{
			ID:           record.ID,
			Name:         firstNonEmpty(record.Name, workspaceDisplayName(record.ID)),
			OutputID:     record.OutputID,
			Active:       record.ID == activeWorkspaceID,
			SurfaceOrder: []string{},
		},
		zones: map[string]*LayoutZone{
			"primary":   {ID: "primary", Name: "Primary", Kind: "work", SurfaceIDs: []string{}},
			"secondary": {ID: "secondary", Name: "Secondary", Kind: "work", SurfaceIDs: []string{}},
			"transient": {ID: "transient", Name: "Transient", Kind: "floating", SurfaceIDs: []string{}},
		},
		zoneOrder: []string{"primary", "secondary", "transient"},
	}
}

func zoneKindForSurface(surface TrackedSurface) string {
	role := SurfaceLayoutRole(surface.LayoutRole)
	if role == SurfaceLayoutRoleFloating || role == SurfaceLayoutRoleTransient {
		return "floating"
	}
	return "work"
}

func (bridge *Bridge) applyWorkspaceAuthorityToBackendLayoutLocked() {
	if bridge.backendLayout == nil {
		return
	}
	layout := cloneLayoutState(*bridge.backendLayout)
	bridge.applyWorkspaceAuthorityToLayoutLocked(&layout)
	bridge.backendLayout = cloneLayoutStatePtr(layout)
}

func (bridge *Bridge) applyWorkspaceAuthorityToLayoutLocked(layout *LayoutState) {
	activeWorkspaceID := bridge.activeWorkspaceIDLocked()
	for _, surface := range bridge.surfaces {
		if surface.Surface.SurfaceKind == SurfaceKindLayer {
			continue
		}
		bridge.ensureWorkspaceLocked(firstNonEmpty(surface.WorkspaceID, surface.Surface.WorkspaceID, activeWorkspaceID))
	}
	for index := range layout.Surfaces {
		surface := &layout.Surfaces[index]
		if surface.WorkspaceID == "" {
			surface.WorkspaceID = activeWorkspaceID
		}
		bridge.ensureWorkspaceLocked(surface.WorkspaceID)
		if tracked, ok := bridge.surfaces[surface.SurfaceID]; ok {
			surface.WorkspaceID = firstNonEmpty(tracked.WorkspaceID, tracked.Surface.WorkspaceID, surface.WorkspaceID)
			surface.Visible = tracked.Visible && surface.WorkspaceID == activeWorkspaceID
			surface.Focused = surface.WorkspaceID == activeWorkspaceID && bridge.layoutFocusedLocked(tracked)
		} else {
			surface.Visible = surface.Visible && surface.WorkspaceID == activeWorkspaceID
			surface.Focused = surface.Focused && surface.WorkspaceID == activeWorkspaceID
		}
	}
	normalizeLayoutState(layout)
	for _, workspaceID := range bridge.workspaceOrderLocked() {
		ensureWorkspace(layout, workspaceID)
	}
	activeSeen := false
	for index := range layout.Workspaces {
		workspace := &layout.Workspaces[index]
		record := bridge.ensureWorkspaceLocked(workspace.ID)
		workspace.Name = firstNonEmpty(record.Name, workspace.Name, workspaceDisplayName(workspace.ID))
		workspace.Active = workspace.ID == activeWorkspaceID
		if workspace.Active {
			activeSeen = true
		}
	}
	if !activeSeen {
		workspace := ensureWorkspace(layout, activeWorkspaceID)
		workspace.Active = true
	}
	bridge.sortLayoutWorkspacesLocked(layout)
}

func (bridge *Bridge) sortLayoutWorkspacesLocked(layout *LayoutState) {
	rank := bridge.workspaceRankLocked()
	sort.SliceStable(layout.Workspaces, func(i, j int) bool {
		leftRank, leftOK := rank[layout.Workspaces[i].ID]
		rightRank, rightOK := rank[layout.Workspaces[j].ID]
		if leftOK && rightOK {
			return leftRank < rightRank
		}
		if leftOK != rightOK {
			return leftOK
		}
		return layout.Workspaces[i].ID < layout.Workspaces[j].ID
	})
}

func normalizeLayoutState(layout *LayoutState) {
	if layout.Mode == "" {
		layout.Mode = LayoutModeZones
	}
	if layout.Settings.Rule == "" {
		layout.Settings = DefaultLayoutSettings()
		layout.Settings.Mode = layout.Mode
	}
	for index := range layout.Workspaces {
		workspace := &layout.Workspaces[index]
		if workspace.ID == "" {
			workspace.ID = defaultWorkspaceID
		}
		if workspace.Name == "" {
			workspace.Name = workspace.ID
		}
	}
	if len(layout.Workspaces) == 0 {
		layout.Workspaces = []LayoutWorkspace{{ID: defaultWorkspaceID, Name: workspaceDisplayName(defaultWorkspaceID), Active: true}}
	}
	for index := range layout.Workspaces {
		workspace := &layout.Workspaces[index]
		workspace.SurfaceOrder = uniqueStrings(workspace.SurfaceOrder)
		if workspace.SurfaceOrder == nil {
			workspace.SurfaceOrder = []string{}
		}
		for zoneIndex := range workspace.Zones {
			workspace.Zones[zoneIndex].SurfaceIDs = uniqueStrings(workspace.Zones[zoneIndex].SurfaceIDs)
			if workspace.Zones[zoneIndex].SurfaceIDs == nil {
				workspace.Zones[zoneIndex].SurfaceIDs = []string{}
			}
		}
	}
	for index := range layout.Surfaces {
		surface := &layout.Surfaces[index]
		if surface.SurfaceID == "" {
			continue
		}
		if surface.Label == "" {
			surface.Label = fmt.Sprintf("%d", index+1)
		}
		if surface.WorkspaceID == "" {
			surface.WorkspaceID = defaultWorkspaceID
		}
		if surface.ZoneID == "" {
			surface.ZoneID = "primary"
		}
		if surface.Mode == "" || !validLayoutMode(surface.Mode) {
			surface.Mode = layout.Mode
		}
		if surface.Participation == "" {
			surface.Participation = SurfaceLayoutRoleTiled
		}
		surface.Floating = surface.Participation == SurfaceLayoutRoleFloating
		surface.Order = index
		workspace := ensureWorkspace(layout, surface.WorkspaceID)
		if workspace.OutputID == "" {
			workspace.OutputID = surface.OutputID
		}
		removeSurfaceFromOtherZones(workspace, surface.ZoneID, surface.SurfaceID)
		if !containsString(workspace.SurfaceOrder, surface.SurfaceID) {
			workspace.SurfaceOrder = append(workspace.SurfaceOrder, surface.SurfaceID)
		}
		ensureZoneContains(workspace, surface.ZoneID, surface.SurfaceID)
	}
}

func ensureWorkspace(layout *LayoutState, workspaceID string) *LayoutWorkspace {
	for index := range layout.Workspaces {
		if layout.Workspaces[index].ID == workspaceID {
			return &layout.Workspaces[index]
		}
	}
	layout.Workspaces = append(layout.Workspaces, LayoutWorkspace{
		ID:     workspaceID,
		Name:   workspaceID,
		Active: len(layout.Workspaces) == 0,
	})
	return &layout.Workspaces[len(layout.Workspaces)-1]
}

func ensureZoneContains(workspace *LayoutWorkspace, zoneID string, surfaceID string) {
	for index := range workspace.Zones {
		if workspace.Zones[index].ID == zoneID {
			if !containsString(workspace.Zones[index].SurfaceIDs, surfaceID) {
				workspace.Zones[index].SurfaceIDs = append(workspace.Zones[index].SurfaceIDs, surfaceID)
			}
			return
		}
	}
	workspace.Zones = append(workspace.Zones, LayoutZone{
		ID:         zoneID,
		Name:       zoneID,
		Kind:       "work",
		SurfaceIDs: []string{surfaceID},
	})
}

func removeSurfaceFromOtherZones(workspace *LayoutWorkspace, zoneID string, surfaceID string) {
	for index := range workspace.Zones {
		if workspace.Zones[index].ID == zoneID {
			continue
		}
		workspace.Zones[index].SurfaceIDs = filterString(workspace.Zones[index].SurfaceIDs, surfaceID)
	}
}

func containsString(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}

func uniqueStrings(values []string) []string {
	if len(values) < 2 {
		return values
	}
	seen := map[string]bool{}
	unique := values[:0]
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		unique = append(unique, value)
	}
	return unique
}

func cloneLayoutStatePtr(layout LayoutState) *LayoutState {
	cloned := cloneLayoutState(layout)
	return &cloned
}

func cloneLayoutState(layout LayoutState) LayoutState {
	cloned := LayoutState{
		Mode:       layout.Mode,
		Revision:   layout.Revision,
		Settings:   layout.Settings,
		Surfaces:   make([]LayoutSurface, len(layout.Surfaces)),
		Workspaces: make([]LayoutWorkspace, len(layout.Workspaces)),
	}
	for index, surface := range layout.Surfaces {
		cloned.Surfaces[index] = surface
		cloned.Surfaces[index].Geometry = cloneGeometry(surface.Geometry)
	}
	for index, workspace := range layout.Workspaces {
		cloned.Workspaces[index] = LayoutWorkspace{
			ID:           workspace.ID,
			Name:         workspace.Name,
			OutputID:     workspace.OutputID,
			Active:       workspace.Active,
			Zones:        make([]LayoutZone, len(workspace.Zones)),
			SurfaceOrder: append([]string(nil), workspace.SurfaceOrder...),
		}
		for zoneIndex, zone := range workspace.Zones {
			cloned.Workspaces[index].Zones[zoneIndex] = LayoutZone{
				ID:         zone.ID,
				Name:       zone.Name,
				Kind:       zone.Kind,
				SurfaceIDs: append([]string(nil), zone.SurfaceIDs...),
			}
		}
	}
	return cloned
}

func cloneGeometry(geometry *SurfaceGeometry) *SurfaceGeometry {
	if geometry == nil {
		return nil
	}
	cloned := *geometry
	return &cloned
}

func firstGeometry(surface TrackedSurface) *SurfaceGeometry {
	if surface.Geometry != nil {
		return surface.Geometry
	}
	return surface.Surface.Geometry
}

func validLayoutMode(mode LayoutMode) bool {
	switch mode {
	case LayoutModeFreeform, LayoutModeZones, LayoutModeColumns:
		return true
	default:
		return false
	}
}

func (bridge *Bridge) startFocusWaiter(surfaceID string) (*pluginSession, string, chan pluginResponse, error) {
	bridge.mu.Lock()
	defer bridge.mu.Unlock()
	if bridge.plugin == nil {
		return nil, "", nil, classifiedError{class: ErrorCompositorUnavailable, message: "no plugin connected"}
	}
	bridge.focusSeq++
	requestID := fmt.Sprintf("focus-%d-%d", time.Now().UnixNano(), bridge.focusSeq)
	waiter := make(chan pluginResponse, 1)
	bridge.focusWaiters[requestID] = waiter
	bridge.focusWaiterSession[requestID] = bridge.plugin.id
	return bridge.plugin, requestID, waiter, nil
}

func (bridge *Bridge) clearFocusWaiter(requestID string) {
	bridge.mu.Lock()
	defer bridge.mu.Unlock()
	delete(bridge.focusWaiters, requestID)
	delete(bridge.focusWaiterSession, requestID)
}

func (bridge *Bridge) startPlaceWaiter(surfaceID string) (*pluginSession, string, chan pluginResponse, error) {
	bridge.mu.Lock()
	defer bridge.mu.Unlock()
	if bridge.plugin == nil {
		return nil, "", nil, classifiedError{class: ErrorCompositorUnavailable, message: "no plugin connected"}
	}
	bridge.placeSeq++
	requestID := fmt.Sprintf("place-%d-%d", time.Now().UnixNano(), bridge.placeSeq)
	waiter := make(chan pluginResponse, 1)
	bridge.placeWaiters[requestID] = waiter
	bridge.placeWaiterSession[requestID] = bridge.plugin.id
	return bridge.plugin, requestID, waiter, nil
}

func (bridge *Bridge) clearPlaceWaiter(requestID string) {
	bridge.mu.Lock()
	defer bridge.mu.Unlock()
	delete(bridge.placeWaiters, requestID)
	delete(bridge.placeWaiterSession, requestID)
}

func (bridge *Bridge) startStateWaiter(surfaceID string) (*pluginSession, string, chan pluginResponse, error) {
	bridge.mu.Lock()
	defer bridge.mu.Unlock()
	if bridge.plugin == nil {
		return nil, "", nil, classifiedError{class: ErrorCompositorUnavailable, message: "no plugin connected"}
	}
	bridge.stateSeq++
	requestID := fmt.Sprintf("state-%d-%d", time.Now().UnixNano(), bridge.stateSeq)
	waiter := make(chan pluginResponse, 1)
	bridge.stateWaiters[requestID] = waiter
	bridge.stateWaiterSession[requestID] = bridge.plugin.id
	return bridge.plugin, requestID, waiter, nil
}

func (bridge *Bridge) clearStateWaiter(requestID string) {
	bridge.mu.Lock()
	defer bridge.mu.Unlock()
	delete(bridge.stateWaiters, requestID)
	delete(bridge.stateWaiterSession, requestID)
}

func (bridge *Bridge) installPlugin(session *pluginSession) *pluginSession {
	bridge.mu.Lock()
	defer bridge.mu.Unlock()
	bridge.pluginSeq++
	session.id = bridge.pluginSeq
	previous := bridge.plugin
	bridge.plugin = session
	return previous
}

func (bridge *Bridge) clearPlugin(session *pluginSession) {
	bridge.mu.Lock()
	defer bridge.mu.Unlock()
	if bridge.plugin == session {
		bridge.plugin = nil
	}
	bridge.failPluginWaitersLocked(session.id, "plugin disconnected")
}

func (bridge *Bridge) failPluginWaitersLocked(sessionID uint64, message string) {
	response := pluginResponse{OK: false, Error: message, ErrorClass: ErrorCompositorUnavailable}
	for requestID, waiter := range bridge.focusWaiters {
		if bridge.focusWaiterSession[requestID] != sessionID {
			continue
		}
		select {
		case waiter <- response:
		default:
		}
		delete(bridge.focusWaiters, requestID)
		delete(bridge.focusWaiterSession, requestID)
	}
	for requestID, waiter := range bridge.placeWaiters {
		if bridge.placeWaiterSession[requestID] != sessionID {
			continue
		}
		select {
		case waiter <- response:
		default:
		}
		delete(bridge.placeWaiters, requestID)
		delete(bridge.placeWaiterSession, requestID)
	}
	for requestID, waiter := range bridge.stateWaiters {
		if bridge.stateWaiterSession[requestID] != sessionID {
			continue
		}
		select {
		case waiter <- response:
		default:
		}
		delete(bridge.stateWaiters, requestID)
		delete(bridge.stateWaiterSession, requestID)
	}
}

func (bridge *Bridge) sendPluginMessage(session *pluginSession, message any) error {
	bridge.mu.RLock()
	current := bridge.plugin == session
	bridge.mu.RUnlock()
	if !current {
		return classifiedError{class: ErrorCompositorUnavailable, message: "plugin connection changed before request could be sent"}
	}
	if err := session.Send(message); err != nil {
		return classifiedError{class: ErrorCompositorUnavailable, message: "plugin connection unavailable: " + err.Error()}
	}
	return nil
}

func (bridge *Bridge) pluginPeerAllowed(conn net.Conn) bool {
	if bridge.allowedPluginUID == 0 {
		return true
	}
	unixConn, ok := conn.(*net.UnixConn)
	if !ok {
		return true
	}
	raw, err := unixConn.SyscallConn()
	if err != nil {
		return true
	}
	allowed := true
	controlErr := raw.Control(func(fd uintptr) {
		cred, err := syscall.GetsockoptUcred(int(fd), syscall.SOL_SOCKET, syscall.SO_PEERCRED)
		if err != nil {
			return
		}
		allowed = cred.Uid == 0 || uint32(cred.Uid) == bridge.allowedPluginUID
	})
	return controlErr != nil || allowed
}

type pluginSession struct {
	id   uint64
	conn net.Conn
	enc  *json.Encoder
	mu   sync.Mutex
}

func (session *pluginSession) Send(message any) error {
	session.mu.Lock()
	defer session.mu.Unlock()
	return session.enc.Encode(message)
}

func (session *pluginSession) Close() error {
	return session.conn.Close()
}

type pluginResponse struct {
	OK         bool
	Error      string
	ErrorClass string
	Geometry   *SurfaceGeometry
}

func writeResponse(writer io.Writer, response Response) {
	if err := json.NewEncoder(writer).Encode(response); err != nil {
		log.Printf("write compositor response: %v", err)
	}
}

func marshalBody(value any) (json.RawMessage, error) {
	data, err := json.Marshal(value)
	return data, err
}

func decodeBody(data json.RawMessage, value any) error {
	if len(data) == 0 || string(data) == "null" {
		data = json.RawMessage(`{}`)
	}
	if err := json.Unmarshal(data, value); err != nil {
		return fmt.Errorf("bad body: %w", err)
	}
	return nil
}

type classifiedError struct {
	class   string
	message string
}

func (err classifiedError) Error() string { return err.message }

func classifyError(err error) (string, string) {
	var classified classifiedError
	if errors.As(err, &classified) {
		return classified.class, classified.message
	}
	message := err.Error()
	if strings.Contains(message, "not found") && strings.Contains(message, "surface") {
		return ErrorSurfaceNotFound, message
	}
	return ErrorProtocol, message
}

func inspectPNG(data []byte) (*ArtifactVisualInspection, error) {
	imageData, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	return inspectImage(imageData), nil
}

func inspectImage(img image.Image) *ArtifactVisualInspection {
	bounds := img.Bounds()
	unique := map[color.RGBA]struct{}{}
	allBlack := true
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, a := img.At(x, y).RGBA()
			c := color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: uint8(a >> 8)}
			if len(unique) <= 16 {
				unique[c] = struct{}{}
			}
			if c.A != 0 && (c.R != 0 || c.G != 0 || c.B != 0) {
				allBlack = false
			}
		}
	}
	status := "visible"
	classification := ""
	if allBlack || len(unique) <= 1 {
		status = "blank"
		classification = "blank"
	}
	return &ArtifactVisualInspection{
		Status:              status,
		Classification:      classification,
		Width:               bounds.Dx(),
		Height:              bounds.Dy(),
		Mode:                "RGBA",
		UniqueColorsSampled: len(unique),
	}
}
