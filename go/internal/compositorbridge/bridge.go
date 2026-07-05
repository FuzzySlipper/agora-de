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
	MethodListSurfaces       = "list_surfaces"
	MethodListOutputs        = "list_outputs"
	MethodCaptureOutput      = "capture_output"
	MethodGetLayout          = "get_layout"
	MethodSetLayoutMode      = "set_layout_mode"
	MethodFocusSurface       = "focus_surface"
	MethodCloseSurface       = "close_surface"
	MethodMoveResizeSurface  = "move_resize_surface"
	MethodTileSurface        = "tile_surface"
	MethodSetSurfaceFloating = "set_surface_floating"
	MethodAssignSurfaceZone  = "assign_surface_zone"
	MethodMaximizeSurface    = "maximize_surface"
	MethodMinimizeSurface    = "minimize_surface"
	MethodFullscreenSurface  = "fullscreen_surface"
	MethodActivateWorkspace  = "activate_workspace"
)

const (
	PluginSurfaceEvent         = "surface_event"
	PluginFocusSurface         = "focus_surface"
	PluginFocusResponse        = "focus_response"
	PluginCloseSurface         = "close_surface"
	PluginPolicyReplace        = "policy_replace"
	PluginInputContext         = "input_context"
	EventMapped                = "mapped"
	EventUnmapped              = "unmapped"
	EventFocused               = "focused"
	EventFrameDone             = "frame_done"
	EventContentCommit         = "content_committed"
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

type Config struct {
	AllowedPluginUID uint32
}

type Bridge struct {
	allowedPluginUID uint32

	mu           sync.RWMutex
	plugin       *pluginSession
	surfaces     map[string]TrackedSurface
	stale        map[string]time.Time
	focusSeq     uint64
	focusWaiters map[string]chan pluginResponse
	captureSeq   uint64
	layoutSeq    uint64
}

func New(config Config) *Bridge {
	return &Bridge{
		allowedPluginUID: config.AllowedPluginUID,
		surfaces:         map[string]TrackedSurface{},
		stale:            map[string]time.Time{},
		focusWaiters:     map[string]chan pluginResponse{},
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
	bridge.mu.RLock()
	defer bridge.mu.RUnlock()
	surfaces := make([]TrackedSurface, 0, len(bridge.surfaces))
	for _, surface := range bridge.surfaces {
		surfaces = append(surfaces, surface)
	}
	sort.Slice(surfaces, func(i, j int) bool { return surfaces[i].Surface.ID < surfaces[j].Surface.ID })
	return surfaces
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
	bridge.mu.RLock()
	defer bridge.mu.RUnlock()
	return GetLayoutResponse{Layout: bridge.layoutLocked()}
}

func (bridge *Bridge) SetLayoutMode(request SetLayoutModeRequest) (LayoutActionResponse, error) {
	if !validLayoutMode(request.Mode) {
		return LayoutActionResponse{}, fmt.Errorf("unsupported layout mode %q", request.Mode)
	}
	return LayoutActionResponse{}, classifiedError{class: ErrorBackendUnsupported, message: "layout mode changes require compositor backend geometry authority"}
}

func (bridge *Bridge) MoveResizeSurface(request SurfaceLayoutActionRequest) (LayoutActionResponse, error) {
	if request.Geometry == nil {
		return LayoutActionResponse{}, fmt.Errorf("geometry is required")
	}
	if request.Geometry.Width <= 0 || request.Geometry.Height <= 0 {
		return LayoutActionResponse{}, fmt.Errorf("geometry width and height must be positive")
	}
	return bridge.unsupportedSurfaceLayoutAction("surface.move_resize", request.SurfaceID)
}

func (bridge *Bridge) TileSurface(request SurfaceLayoutActionRequest) (LayoutActionResponse, error) {
	if strings.TrimSpace(request.ZoneID) == "" {
		return LayoutActionResponse{}, fmt.Errorf("zone_id is required")
	}
	return bridge.unsupportedSurfaceLayoutAction("surface.tile", request.SurfaceID)
}

func (bridge *Bridge) SetSurfaceFloating(request SurfaceLayoutActionRequest) (LayoutActionResponse, error) {
	if request.Floating == nil {
		return LayoutActionResponse{}, fmt.Errorf("floating is required")
	}
	return bridge.unsupportedSurfaceLayoutAction("surface.set_floating", request.SurfaceID)
}

func (bridge *Bridge) AssignSurfaceZone(request SurfaceLayoutActionRequest) (LayoutActionResponse, error) {
	if strings.TrimSpace(request.ZoneID) == "" {
		return LayoutActionResponse{}, fmt.Errorf("zone_id is required")
	}
	return bridge.unsupportedSurfaceLayoutAction("surface.assign_zone", request.SurfaceID)
}

func (bridge *Bridge) MaximizeSurface(request SurfaceLayoutActionRequest) (LayoutActionResponse, error) {
	return bridge.unsupportedSurfaceLayoutAction("surface.maximize", request.SurfaceID)
}

func (bridge *Bridge) MinimizeSurface(request SurfaceLayoutActionRequest) (LayoutActionResponse, error) {
	return bridge.unsupportedSurfaceLayoutAction("surface.minimize", request.SurfaceID)
}

func (bridge *Bridge) FullscreenSurface(request SurfaceLayoutActionRequest) (LayoutActionResponse, error) {
	return bridge.unsupportedSurfaceLayoutAction("surface.fullscreen", request.SurfaceID)
}

func (bridge *Bridge) ActivateWorkspace(request WorkspaceActionRequest) (LayoutActionResponse, error) {
	if strings.TrimSpace(request.WorkspaceID) == "" {
		return LayoutActionResponse{}, fmt.Errorf("workspace_id is required")
	}
	bridge.mu.RLock()
	layout := bridge.layoutLocked()
	bridge.mu.RUnlock()
	for _, workspace := range layout.Workspaces {
		if workspace.ID == request.WorkspaceID {
			return LayoutActionResponse{}, classifiedError{class: ErrorBackendUnsupported, message: "workspace activation requires compositor backend workspace authority"}
		}
	}
	return LayoutActionResponse{}, classifiedError{class: ErrorSurfaceNotFound, message: fmt.Sprintf("workspace %s not found", request.WorkspaceID)}
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

	if err := session.Send(map[string]any{"type": PluginFocusSurface, "request_id": requestID, "surface_id": request.SurfaceID}); err != nil {
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
			return SurfaceActionResponse{}, classifiedError{class: ErrorProtocol, message: message}
		}
	case <-time.After(timeout):
		return SurfaceActionResponse{}, classifiedError{class: ErrorFrameTimeout, message: "focus request timed out"}
	}

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
	if err := session.Send(map[string]any{"type": PluginCloseSurface, "surface_id": request.SurfaceID}); err != nil {
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
	case PluginFocusResponse:
		bridge.handleFocusResponse(event)
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

	bridge.mu.Lock()
	defer bridge.mu.Unlock()
	if event.Event == EventUnmapped {
		bridge.stale[event.Surface.ID] = now
		delete(bridge.surfaces, event.Surface.ID)
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
	applyLayoutDefaults(&tracked)
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
	surface.WorkspaceID = firstNonEmpty(surface.WorkspaceID, "workspace-1")
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

func (bridge *Bridge) outputsLocked() map[string]LogicalOutput {
	outputs := map[string]LogicalOutput{}
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
		sort.Strings(output.Surfaces)
		outputs[name] = output
	}
	return outputs
}

func (bridge *Bridge) layoutLocked() LayoutState {
	surfaces := make([]TrackedSurface, 0, len(bridge.surfaces))
	for _, surface := range bridge.surfaces {
		if surface.Surface.SurfaceKind == SurfaceKindLayer {
			continue
		}
		surfaces = append(surfaces, surface)
	}
	sort.Slice(surfaces, func(i, j int) bool { return surfaces[i].Surface.ID < surfaces[j].Surface.ID })

	zones := map[string]*LayoutZone{
		"primary":   {ID: "primary", Name: "Primary", Kind: "work"},
		"secondary": {ID: "secondary", Name: "Secondary", Kind: "work"},
		"transient": {ID: "transient", Name: "Transient", Kind: "floating"},
	}
	zoneOrder := []string{"primary", "secondary", "transient"}
	layoutSurfaces := make([]LayoutSurface, 0, len(surfaces))
	surfaceOrder := make([]string, 0, len(surfaces))
	outputID := ""
	mode := LayoutModeFreeform
	for index, surface := range surfaces {
		workspaceID := firstNonEmpty(surface.WorkspaceID, "workspace-1")
		zoneID := firstNonEmpty(surface.ZoneID, "primary")
		if _, ok := zones[zoneID]; !ok {
			zones[zoneID] = &LayoutZone{ID: zoneID, Name: zoneID, Kind: "work"}
			zoneOrder = append(zoneOrder, zoneID)
		}
		zones[zoneID].SurfaceIDs = append(zones[zoneID].SurfaceIDs, surface.Surface.ID)
		surfaceOrder = append(surfaceOrder, surface.Surface.ID)
		if outputID == "" {
			outputID = firstNonEmpty(surface.OutputID, surface.Surface.OutputID)
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
			Focused:       surface.Focused,
			Visible:       surface.Visible,
			Geometry:      firstGeometry(surface),
			Order:         index,
		})
	}

	layoutZones := make([]LayoutZone, 0, len(zoneOrder))
	for _, zoneID := range zoneOrder {
		zone := zones[zoneID]
		layoutZones = append(layoutZones, *zone)
	}
	workspace := LayoutWorkspace{
		ID:           "workspace-1",
		Name:         "workspace 1",
		OutputID:     outputID,
		Active:       true,
		Zones:        layoutZones,
		SurfaceOrder: surfaceOrder,
	}
	return LayoutState{
		Mode:       mode,
		Revision:   bridge.layoutSeq,
		Surfaces:   layoutSurfaces,
		Workspaces: []LayoutWorkspace{workspace},
	}
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
	return bridge.plugin, requestID, waiter, nil
}

func (bridge *Bridge) clearFocusWaiter(requestID string) {
	bridge.mu.Lock()
	defer bridge.mu.Unlock()
	delete(bridge.focusWaiters, requestID)
}

func (bridge *Bridge) installPlugin(session *pluginSession) *pluginSession {
	bridge.mu.Lock()
	defer bridge.mu.Unlock()
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
	OK    bool
	Error string
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
