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
	MethodListSurfaces  = "list_surfaces"
	MethodListOutputs   = "list_outputs"
	MethodCaptureOutput = "capture_output"
	MethodFocusSurface  = "focus_surface"
	MethodCloseSurface  = "close_surface"
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
	if event.Event == EventFrameDone {
		tracked.FrameCount++
		tracked.LastPresentTimestamp = &now
	}
	if event.Event == EventContentCommit {
		tracked.ContentCommitCount++
		tracked.LastContentCommitTimestamp = &now
	}
	delete(bridge.stale, event.Surface.ID)
	bridge.surfaces[event.Surface.ID] = tracked
}

func mergeSurfaceReadback(next *TrackedSurface, previous TrackedSurface, event string, now time.Time) {
	if next.Geometry == nil {
		next.Geometry = previous.Geometry
	}
	if next.PixelSize == nil {
		next.PixelSize = previous.PixelSize
	}
	if next.OutputID == "" {
		next.OutputID = previous.OutputID
		next.Surface.OutputID = previous.OutputID
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
