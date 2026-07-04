package surfaces

import (
	"sort"

	"agora-de.local/go/internal/surfacetrack"
)

type SurfaceView struct {
	ID                 string `json:"id"`
	OwnerUID           int    `json:"ownerUid"`
	Mapped             bool   `json:"mapped"`
	Focused            bool   `json:"focused"`
	InputDeniedCount   int    `json:"inputDeniedCount"`
	FrameCount         int    `json:"frameCount,omitempty"`
	ContentCommitCount int    `json:"contentCommitCount,omitempty"`
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
