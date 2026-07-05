package surfaces

import (
	"sort"

	"agora-de.local/go/internal/surfacetrack"
)

type SurfaceView struct {
	ID                 string        `json:"id"`
	Label              string        `json:"label,omitempty"`
	AppID              string        `json:"appId,omitempty"`
	Title              string        `json:"title,omitempty"`
	Role               string        `json:"role,omitempty"`
	SurfaceKind        string        `json:"surfaceKind,omitempty"`
	LaunchID           string        `json:"launchId,omitempty"`
	OwnerUID           int           `json:"ownerUid"`
	Mapped             bool          `json:"mapped"`
	Focused            bool          `json:"focused"`
	Visible            bool          `json:"visible"`
	OutputID           string        `json:"outputId,omitempty"`
	WorkspaceID        string        `json:"workspaceId,omitempty"`
	ZoneID             string        `json:"zoneId,omitempty"`
	LayoutMode         string        `json:"layoutMode,omitempty"`
	LayoutRole         string        `json:"layoutRole,omitempty"`
	Geometry           *GeometryView `json:"geometry,omitempty"`
	InputDeniedCount   int           `json:"inputDeniedCount"`
	FrameCount         int           `json:"frameCount,omitempty"`
	ContentCommitCount int           `json:"contentCommitCount,omitempty"`
}

type GeometryView struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

func LifecycleViews(source *surfacetrack.Projection) []SurfaceView {
	if source == nil {
		return []SurfaceView{}
	}

	states := source.Surfaces()
	sort.Slice(states, func(left, right int) bool {
		return states[left].ID < states[right].ID
	})

	views := make([]SurfaceView, 0, len(states))
	for _, state := range states {
		views = append(views, SurfaceView{
			ID:               state.ID,
			OwnerUID:         state.OwnerUID,
			Mapped:           state.Mapped,
			Focused:          state.Focused,
			InputDeniedCount: state.DeniedInputs,
		})
	}
	return views
}
