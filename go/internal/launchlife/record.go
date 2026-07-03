package launchlife

import (
	"errors"
	"strings"

	"agora-de.local/go/internal/session"
)

var ErrInvalidLaunch = errors.New("invalid launch")

type State string

const (
	StatePending       State = "pending"
	StateSurfaceMapped State = "surface_mapped"
	StateTerminated    State = "terminated"
)

type Record struct {
	ID           string
	RequesterUID int
	SessionToken session.Token
	State        State
	SurfaceID    string
}

func NewRecord(id string, requesterUID int, token session.Token) (Record, error) {
	id = strings.TrimSpace(id)
	if id == "" || requesterUID == 0 || token == "" {
		return Record{}, ErrInvalidLaunch
	}
	return Record{
		ID:           id,
		RequesterUID: requesterUID,
		SessionToken: token,
		State:        StatePending,
	}, nil
}

func (record Record) WithMappedSurface(surfaceID string) (Record, error) {
	surfaceID = strings.TrimSpace(surfaceID)
	if surfaceID == "" {
		return Record{}, ErrInvalidLaunch
	}
	record.SurfaceID = surfaceID
	record.State = StateSurfaceMapped
	return record, nil
}

func (record Record) WithTerminated() Record {
	record.State = StateTerminated
	return record
}

