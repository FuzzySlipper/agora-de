package compositorbridge

import (
	"encoding/json"
	"time"
)

type Request struct {
	Method string          `json:"method"`
	Body   json.RawMessage `json:"body"`
}

type Response struct {
	OK           bool            `json:"ok"`
	Body         json.RawMessage `json:"body,omitempty"`
	ErrorClass   string          `json:"error_class,omitempty"`
	ErrorMessage string          `json:"error_message,omitempty"`
}

type SurfaceGeometry struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

type LayerShellSurfaceMetadata struct {
	Namespace     string   `json:"namespace,omitempty"`
	Layer         string   `json:"layer,omitempty"`
	Anchors       []string `json:"anchors,omitempty"`
	ExclusiveZone *bool    `json:"exclusive_zone,omitempty"`
	EffectiveRole string   `json:"effective_role,omitempty"`
	HelperRole    string   `json:"helper_role,omitempty"`
}

type CompositorSurface struct {
	ID            string                     `json:"id"`
	WayfireViewID uint32                     `json:"wayfire_view_id"`
	SurfaceKind   string                     `json:"surface_kind,omitempty"`
	AppID         string                     `json:"app_id,omitempty"`
	Title         string                     `json:"title,omitempty"`
	Role          string                     `json:"role,omitempty"`
	Label         string                     `json:"label,omitempty"`
	LayerShell    *LayerShellSurfaceMetadata `json:"layer_shell,omitempty"`
	Geometry      *SurfaceGeometry           `json:"geometry,omitempty"`
	PixelSize     *SurfaceGeometry           `json:"pixel_size,omitempty"`
	ScaleFactor   float64                    `json:"scale_factor,omitempty"`
	Visible       *bool                      `json:"visible,omitempty"`
	OutputID      string                     `json:"output_id,omitempty"`
	WorkspaceID   string                     `json:"workspace_id,omitempty"`
	ZoneID        string                     `json:"zone_id,omitempty"`
	LayoutMode    string                     `json:"layout_mode,omitempty"`
	LayoutRole    string                     `json:"layout_role,omitempty"`
}

type ClientIdentity struct {
	PID int32  `json:"pid"`
	UID uint32 `json:"uid"`
	GID uint32 `json:"gid"`
}

type TrackedSurface struct {
	Surface                    CompositorSurface `json:"surface"`
	Client                     ClientIdentity    `json:"client"`
	LastEvent                  string            `json:"last_event"`
	Device                     string            `json:"device,omitempty"`
	UpdatedAt                  time.Time         `json:"updated_at"`
	Geometry                   *SurfaceGeometry  `json:"geometry,omitempty"`
	Focused                    bool              `json:"focused"`
	PixelSize                  *SurfaceGeometry  `json:"pixel_size,omitempty"`
	ScaleFactor                float64           `json:"scale_factor,omitempty"`
	Capturable                 bool              `json:"capturable"`
	InputInjectable            bool              `json:"input_injectable"`
	FrameCount                 uint64            `json:"frame_count"`
	LastPresentTimestamp       *time.Time        `json:"last_present_timestamp,omitempty"`
	ContentCommitCount         uint64            `json:"content_commit_count,omitempty"`
	LastContentCommitTimestamp *time.Time        `json:"last_content_commit_timestamp,omitempty"`
	CaptureCount               uint64            `json:"capture_count,omitempty"`
	LastCaptureTimestamp       *time.Time        `json:"last_capture_timestamp,omitempty"`
	Visible                    bool              `json:"visible"`
	OutputID                   string            `json:"output_id,omitempty"`
	WorkspaceID                string            `json:"workspace_id,omitempty"`
	ZoneID                     string            `json:"zone_id,omitempty"`
	LayoutMode                 string            `json:"layout_mode,omitempty"`
	LayoutRole                 string            `json:"layout_role,omitempty"`
	LayoutRevision             uint64            `json:"layout_revision,omitempty"`
}

type ListSurfacesResponse struct {
	Surfaces []TrackedSurface `json:"surfaces,omitempty"`
}

type LayoutMode string

const (
	LayoutModeFreeform LayoutMode = "freeform"
	LayoutModeZones    LayoutMode = "zones"
	LayoutModeColumns  LayoutMode = "columns"
)

type SurfaceLayoutRole string

const (
	SurfaceLayoutRoleTiled     SurfaceLayoutRole = "tiled"
	SurfaceLayoutRoleFloating  SurfaceLayoutRole = "floating"
	SurfaceLayoutRoleTransient SurfaceLayoutRole = "transient"
)

type LayoutSurface struct {
	SurfaceID     string            `json:"surface_id"`
	Label         string            `json:"label"`
	AppID         string            `json:"app_id,omitempty"`
	Title         string            `json:"title,omitempty"`
	Role          string            `json:"role,omitempty"`
	OutputID      string            `json:"output_id,omitempty"`
	WorkspaceID   string            `json:"workspace_id"`
	ZoneID        string            `json:"zone_id"`
	Mode          LayoutMode        `json:"mode"`
	Participation SurfaceLayoutRole `json:"participation"`
	Floating      bool              `json:"floating"`
	Focused       bool              `json:"focused"`
	Visible       bool              `json:"visible"`
	Geometry      *SurfaceGeometry  `json:"geometry,omitempty"`
	Order         int               `json:"order"`
}

type LayoutZone struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Kind       string   `json:"kind"`
	SurfaceIDs []string `json:"surface_ids"`
}

type LayoutWorkspace struct {
	ID           string       `json:"id"`
	Name         string       `json:"name"`
	OutputID     string       `json:"output_id,omitempty"`
	Active       bool         `json:"active"`
	Zones        []LayoutZone `json:"zones"`
	SurfaceOrder []string     `json:"surface_order"`
}

type LayoutSettings struct {
	Rule        string     `json:"rule"`
	Mode        LayoutMode `json:"mode"`
	Gaps        LayoutGaps `json:"gaps"`
	MasterCount int        `json:"master_count"`
	MasterRatio float64    `json:"master_ratio"`
	SmartGaps   bool       `json:"smart_gaps"`
}

type LayoutGaps struct {
	OuterHorizontal int `json:"outer_horizontal"`
	OuterVertical   int `json:"outer_vertical"`
	InnerHorizontal int `json:"inner_horizontal"`
	InnerVertical   int `json:"inner_vertical"`
}

type LayoutState struct {
	Mode       LayoutMode        `json:"mode"`
	Revision   uint64            `json:"revision"`
	Settings   LayoutSettings    `json:"settings"`
	Surfaces   []LayoutSurface   `json:"surfaces,omitempty"`
	Workspaces []LayoutWorkspace `json:"workspaces,omitempty"`
}

type GetLayoutResponse struct {
	Layout LayoutState `json:"layout"`
}

type LayoutStateEvent struct {
	Type   string      `json:"type"`
	Layout LayoutState `json:"layout"`
}

type FocusSurfaceRequest struct {
	SurfaceID     string `json:"surface_id"`
	WaitTimeoutMs int    `json:"wait_timeout_ms,omitempty"`
}

type CloseSurfaceRequest struct {
	SurfaceID     string `json:"surface_id"`
	WaitTimeoutMs int    `json:"timeout_ms,omitempty"`
}

type SetLayoutModeRequest struct {
	Mode LayoutMode `json:"mode"`
}

type UpdateLayoutSettingsRequest struct {
	Rule        *string     `json:"rule,omitempty"`
	Mode        *LayoutMode `json:"mode,omitempty"`
	Gaps        *LayoutGaps `json:"gaps,omitempty"`
	MasterCount *int        `json:"master_count,omitempty"`
	MasterRatio *float64    `json:"master_ratio,omitempty"`
	SmartGaps   *bool       `json:"smart_gaps,omitempty"`
}

type SurfaceLayoutActionRequest struct {
	SurfaceID     string           `json:"surface_id"`
	Geometry      *SurfaceGeometry `json:"geometry,omitempty"`
	WorkspaceID   string           `json:"workspace_id,omitempty"`
	ZoneID        string           `json:"zone_id,omitempty"`
	Floating      *bool            `json:"floating,omitempty"`
	Enabled       *bool            `json:"enabled,omitempty"`
	WaitTimeoutMs int              `json:"wait_timeout_ms,omitempty"`
}

type WorkspaceActionRequest struct {
	WorkspaceID   string `json:"workspace_id"`
	WaitTimeoutMs int    `json:"wait_timeout_ms,omitempty"`
}

type SurfaceActionResponse struct {
	Action           string          `json:"action"`
	SurfaceID        string          `json:"surface_id"`
	Decision         string          `json:"decision"`
	Reason           string          `json:"reason,omitempty"`
	Error            string          `json:"error,omitempty"`
	FocusedSurfaceID string          `json:"focused_surface_id,omitempty"`
	ClosedSurfaceID  string          `json:"closed_surface_id,omitempty"`
	Queued           bool            `json:"queued,omitempty"`
	Surface          *TrackedSurface `json:"surface,omitempty"`
}

type LayoutActionResponse struct {
	Action      string          `json:"action"`
	Decision    string          `json:"decision"`
	Reason      string          `json:"reason,omitempty"`
	SurfaceID   string          `json:"surface_id,omitempty"`
	WorkspaceID string          `json:"workspace_id,omitempty"`
	Layout      *LayoutState    `json:"layout,omitempty"`
	Surface     *TrackedSurface `json:"surface,omitempty"`
}

type LogicalOutput struct {
	Name           string    `json:"name"`
	Width          int       `json:"width"`
	Height         int       `json:"height"`
	Scale          float64   `json:"scale"`
	Mode           string    `json:"mode"`
	PhysicalX      int       `json:"physical_x"`
	PhysicalY      int       `json:"physical_y"`
	PhysicalWidth  int       `json:"physical_width"`
	PhysicalHeight int       `json:"physical_height"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	Surfaces       []string  `json:"surfaces,omitempty"`
}

type ListOutputsResponse struct {
	Outputs []LogicalOutput `json:"outputs,omitempty"`
}

type CaptureOutputRequest struct {
	Name                  string `json:"name"`
	Export                bool   `json:"export,omitempty"`
	SessionID             string `json:"session_id,omitempty"`
	SessionToken          string `json:"session_token,omitempty"`
	AuditCorrelationID    string `json:"audit_correlation_id,omitempty"`
	EvidenceClass         string `json:"evidence_class,omitempty"`
	ASHACommandSequenceID string `json:"asha_command_sequence_id,omitempty"`
}

type CaptureOutputResponse struct {
	Output   string                   `json:"output"`
	Captures []CaptureSurfaceResponse `json:"captures,omitempty"`
	Warnings []string                 `json:"warnings,omitempty"`
}

type CaptureSurfaceResponse struct {
	SurfaceID        string                    `json:"surface_id"`
	RequestID        string                    `json:"request_id,omitempty"`
	Path             string                    `json:"path"`
	ImagePath        string                    `json:"image_path,omitempty"`
	Width            uint32                    `json:"width"`
	Height           uint32                    `json:"height"`
	Format           string                    `json:"format"`
	SHA256           string                    `json:"sha256"`
	CapturedAt       time.Time                 `json:"captured_at,omitempty"`
	VisualInspection *ArtifactVisualInspection `json:"visual_inspection,omitempty"`
	Artifact         *ArtifactRecord           `json:"artifact,omitempty"`
}

type ArtifactRecord struct {
	ArtifactID            string                    `json:"artifact_id"`
	SessionID             string                    `json:"session_id"`
	SurfaceID             string                    `json:"surface_id"`
	RequestID             string                    `json:"request_id"`
	ImagePath             string                    `json:"image_path"`
	IndexPath             string                    `json:"index_path"`
	Width                 uint32                    `json:"width"`
	Height                uint32                    `json:"height"`
	Format                string                    `json:"format"`
	SHA256                string                    `json:"sha256"`
	CaptureBackend        string                    `json:"capture_backend"`
	AuditCorrelationID    string                    `json:"audit_correlation_id,omitempty"`
	EvidenceClass         string                    `json:"evidence_class"`
	Timestamp             time.Time                 `json:"timestamp"`
	ASHACommandSequenceID string                    `json:"asha_command_sequence_id,omitempty"`
	VisualInspection      *ArtifactVisualInspection `json:"visual_inspection,omitempty"`
	Warnings              []string                  `json:"warnings,omitempty"`
}

type ArtifactVisualInspection struct {
	Status              string     `json:"status"`
	Classification      string     `json:"classification,omitempty"`
	Width               int        `json:"width"`
	Height              int        `json:"height"`
	Mode                string     `json:"mode"`
	Extrema             [][2]uint8 `json:"extrema,omitempty"`
	UniqueColorsSampled int        `json:"unique_colors_sampled,omitempty"`
}

type pluginEvent struct {
	Type      string            `json:"type"`
	Event     string            `json:"event,omitempty"`
	Device    string            `json:"device,omitempty"`
	Surface   CompositorSurface `json:"surface"`
	Client    ClientIdentity    `json:"client"`
	Layout    LayoutState       `json:"layout,omitempty"`
	RequestID string            `json:"request_id,omitempty"`
	SurfaceID string            `json:"surface_id,omitempty"`
	Geometry  *SurfaceGeometry  `json:"geometry,omitempty"`
	OK        bool              `json:"ok,omitempty"`
	Error     string            `json:"error,omitempty"`
}
