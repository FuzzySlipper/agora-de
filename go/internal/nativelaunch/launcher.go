package nativelaunch

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"agora-de.local/go/internal/appcatalog"
	"agora-de.local/go/internal/launchlife"
	"agora-de.local/go/internal/session"
)

var (
	ErrInvalidRequest       = errors.New("invalid native launch request")
	ErrUnsupportedFieldCode = errors.New("unsupported desktop-entry field code")
	ErrNoBridge             = errors.New("native launch bridge unavailable")
)

type Status string

const (
	StatusLaunched                    Status = "launched"
	StatusLaunchedWithoutSurface      Status = "launched_without_surface"
	StatusRejected                    Status = "rejected"
	StatusFailed                      Status = "failed"
	StatusTimedOut                    Status = "timed_out"
	StatusTimedOutNoSurface           Status = "timed_out_no_surface"
	StatusSurfaceObservedAfterTimeout Status = "surface_observed_after_timeout"
	StatusReusedExistingWindow        Status = "reused_existing_window"
)

const DefaultWaitTimeout = 5 * time.Second

type Request struct {
	Entry              appcatalog.Entry
	RequesterUID       int
	RequesterGID       int
	SessionToken       session.Token
	AuditCorrelationID string
	OutputName         string
	DesktopFilePath    string
	WorkingDirectory   string
	HomeDirectory      string
	BaseEnvironment    map[string]string
	WaitTimeout        time.Duration
}

type Result struct {
	LaunchID  string
	SurfaceID string
	Status    Status
}

type BridgeRequest struct {
	Args               []string
	Environment        []string
	WorkingDirectory   string
	ExpectedAppID      string
	RequesterUID       int
	RequesterGID       int
	SessionToken       session.Token
	AuditCorrelationID string
	OutputName         string
	WaitTimeout        time.Duration
}

type BridgeResult struct {
	LaunchID  string
	SurfaceID string
	Status    Status
}

type Bridge interface {
	Launch(context.Context, BridgeRequest) (BridgeResult, error)
}

type Launcher struct {
	Bridge Bridge
}

func New(bridge Bridge) Launcher {
	return Launcher{Bridge: bridge}
}

func CanPrepare(entry appcatalog.Entry) bool {
	if !entry.Visible() {
		return false
	}
	_, err := BuildArgv(entry, "/usr/share/applications/"+entry.ID)
	return err == nil
}

func (launcher Launcher) Launch(ctx context.Context, request Request) (Result, error) {
	if launcher.Bridge == nil {
		return Result{Status: StatusRejected}, ErrNoBridge
	}
	if err := validateRequest(request); err != nil {
		return Result{Status: StatusRejected}, err
	}
	args, err := BuildArgv(request.Entry, request.DesktopFilePath)
	if err != nil {
		return Result{Status: StatusRejected}, err
	}
	workingDirectory, err := ResolveWorkingDirectory(request.WorkingDirectory, request.HomeDirectory)
	if err != nil {
		return Result{Status: StatusRejected}, err
	}

	waitTimeout := request.WaitTimeout
	if waitTimeout <= 0 {
		waitTimeout = DefaultWaitTimeout
	}

	bridgeResult, err := launcher.Bridge.Launch(ctx, BridgeRequest{
		Args:               args,
		Environment:        BuildEnvironment(request.BaseEnvironment),
		WorkingDirectory:   workingDirectory,
		ExpectedAppID:      expectedAppID(request.Entry),
		RequesterUID:       request.RequesterUID,
		RequesterGID:       request.RequesterGID,
		SessionToken:       request.SessionToken,
		AuditCorrelationID: request.AuditCorrelationID,
		OutputName:         request.OutputName,
		WaitTimeout:        waitTimeout,
	})
	if err != nil {
		return Result{Status: StatusFailed}, err
	}

	status := bridgeResult.Status
	if status == "" {
		status = StatusLaunched
	}
	if err := validateBridgeResult(request, bridgeResult, status); err != nil {
		return Result{LaunchID: bridgeResult.LaunchID, SurfaceID: bridgeResult.SurfaceID, Status: StatusFailed}, err
	}
	return Result{LaunchID: bridgeResult.LaunchID, SurfaceID: bridgeResult.SurfaceID, Status: status}, nil
}

func validateRequest(request Request) error {
	if !CanPrepare(request.Entry) {
		return fmt.Errorf("%w: entry is not preparable", ErrInvalidRequest)
	}
	if request.RequesterUID <= 0 || request.RequesterGID <= 0 {
		return fmt.Errorf("%w: requester uid/gid required", ErrInvalidRequest)
	}
	if request.SessionToken == "" {
		return fmt.Errorf("%w: session token required", ErrInvalidRequest)
	}
	if request.AuditCorrelationID == "" {
		return fmt.Errorf("%w: audit correlation id required", ErrInvalidRequest)
	}
	return nil
}

func validateBridgeResult(request Request, bridgeResult BridgeResult, status Status) error {
	if bridgeResult.LaunchID == "" {
		return fmt.Errorf("%w: bridge result missing launch id", ErrInvalidRequest)
	}
	if _, err := launchlife.NewRecord(bridgeResult.LaunchID, request.RequesterUID, request.SessionToken); err != nil {
		return err
	}

	switch status {
	case StatusLaunched:
		if bridgeResult.SurfaceID == "" {
			return fmt.Errorf("%w: launched result missing surface id", ErrInvalidRequest)
		}
	case StatusLaunchedWithoutSurface, StatusTimedOut, StatusTimedOutNoSurface, StatusSurfaceObservedAfterTimeout, StatusReusedExistingWindow, StatusFailed:
		return nil
	case StatusRejected:
		return fmt.Errorf("%w: bridge rejected launch", ErrInvalidRequest)
	default:
		return fmt.Errorf("%w: unknown bridge status %q", ErrInvalidRequest, status)
	}
	return nil
}

func expectedAppID(entry appcatalog.Entry) string {
	if value := strings.TrimSpace(entry.StartupWMClass); value != "" {
		return value
	}
	id := strings.TrimSpace(entry.ID)
	if strings.HasSuffix(id, ".desktop") {
		id = strings.TrimSuffix(id, ".desktop")
	}
	return id
}
