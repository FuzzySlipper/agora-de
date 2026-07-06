package nativelaunch

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

type CompositorctlBridge struct {
	Path string
}

func (bridge CompositorctlBridge) Launch(ctx context.Context, request BridgeRequest) (BridgeResult, error) {
	path := strings.TrimSpace(bridge.Path)
	if path == "" {
		path = "compositorctl"
	}

	args := []string{"launch"}
	for _, arg := range request.Args {
		args = append(args, "--arg", arg)
	}
	for _, environment := range request.Environment {
		args = append(args, "--env", environment)
	}
	if request.WorkingDirectory != "" {
		args = append(args, "--cwd", request.WorkingDirectory)
	}
	if request.ExpectedAppID != "" {
		args = append(args, "--expected-app-id", request.ExpectedAppID)
	}
	if request.RequesterUID > 0 {
		args = append(args, "--uid", strconv.Itoa(request.RequesterUID))
	}
	if request.RequesterGID > 0 {
		args = append(args, "--gid", strconv.Itoa(request.RequesterGID))
	}
	if request.SessionToken != "" {
		args = append(args, "--session-token", string(request.SessionToken))
	}
	if request.AuditCorrelationID != "" {
		args = append(args, "--audit-correlation-id", request.AuditCorrelationID)
	}
	if request.OutputName != "" {
		args = append(args, "--output", request.OutputName)
	}
	if request.WaitTimeout > 0 {
		args = append(args, "--wait-surface", "--wait-timeout-ms", strconv.FormatInt(request.WaitTimeout.Milliseconds(), 10))
	}

	output, err := exec.CommandContext(ctx, path, args...).Output()
	if err != nil {
		return BridgeResult{Status: StatusFailed}, fmt.Errorf("compositorctl structured launch: %w", err)
	}
	return decodeCompositorctlLaunch(output)
}

type compositorctlLaunchResponse struct {
	LaunchID  string `json:"launch_id"`
	SurfaceID string `json:"surface_id"`
	Status    Status `json:"status"`
	Surface   struct {
		Surface struct {
			ID string `json:"id"`
		} `json:"surface"`
	} `json:"surface"`
}

func decodeCompositorctlLaunch(payload []byte) (BridgeResult, error) {
	var response compositorctlLaunchResponse
	if err := json.Unmarshal(payload, &response); err != nil {
		return BridgeResult{Status: StatusFailed}, fmt.Errorf("decode compositorctl launch: %w", err)
	}
	surfaceID := response.SurfaceID
	if surfaceID == "" {
		surfaceID = response.Surface.Surface.ID
	}
	status := response.Status
	if status == "" {
		status = StatusLaunched
	}
	return BridgeResult{
		LaunchID:  response.LaunchID,
		SurfaceID: surfaceID,
		Status:    status,
	}, nil
}
