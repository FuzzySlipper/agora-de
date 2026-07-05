package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const defaultReadbackCompositorctl = "/usr/local/bin/compositorctl"

var listSurfacesFunc = listSurfaces

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string, stdout io.Writer, stderr io.Writer) error {
	if len(args) == 0 {
		usage(stderr)
		return errors.New("command is required")
	}
	switch args[0] {
	case "launch":
		return runLaunch(args[1:], stdout)
	default:
		usage(stderr)
		return fmt.Errorf("unsupported command %q", args[0])
	}
}

func usage(output io.Writer) {
	fmt.Fprintln(output, `Usage: compositorctl <command> [flags]

Commands:
  launch   Launch a native process from a structured argv vector`)
}

type repeatedFlag []string

func (flag *repeatedFlag) String() string {
	return strings.Join(*flag, ",")
}

func (flag *repeatedFlag) Set(value string) error {
	*flag = append(*flag, value)
	return nil
}

func runLaunch(args []string, stdout io.Writer) error {
	var argv repeatedFlag
	var environment repeatedFlag
	fs := flag.NewFlagSet("launch", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	cwd := fs.String("cwd", "", "working directory")
	uid := fs.Uint("uid", 0, "requester uid")
	gid := fs.Uint("gid", 0, "requester gid")
	sessionToken := fs.String("session-token", "", "session token")
	auditCorrelationID := fs.String("audit-correlation-id", "", "audit correlation id")
	outputName := fs.String("output", "", "logical output name")
	waitSurface := fs.Bool("wait-surface", false, "wait for a matching mapped surface")
	waitTimeoutMs := fs.Int("wait-timeout-ms", 5000, "surface wait timeout in milliseconds")
	fs.Var(&argv, "arg", "argv element; repeat for each argument")
	fs.Var(&environment, "env", "environment variable KEY=VALUE; repeat as needed")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if len(argv) == 0 {
		return errors.New("launch requires at least one --arg")
	}
	if *sessionToken == "" {
		return errors.New("--session-token is required")
	}
	if *auditCorrelationID == "" {
		return errors.New("--audit-correlation-id is required")
	}

	launchID := fmt.Sprintf("launch-%d", time.Now().UnixNano())
	startedAt := time.Now()
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if *cwd != "" {
		cmd.Dir = *cwd
	}
	if len(environment) > 0 {
		cmd.Env = append([]string(nil), environment...)
	}
	if err := applyCredential(cmd, uint32(*uid), uint32(*gid)); err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	response := launchResponse{
		LaunchID:            launchID,
		PID:                 cmd.Process.Pid,
		Status:              "launched_without_surface",
		SessionTokenPresent: *sessionToken != "",
		AuditCorrelationID:  *auditCorrelationID,
		OutputName:          *outputName,
	}
	if *waitSurface {
		timeout := time.Duration(*waitTimeoutMs) * time.Millisecond
		surface, ok, err := waitForSurface(cmd.Process.Pid, startedAt, timeout, done)
		if err != nil {
			response.Status = "failed"
			_ = json.NewEncoder(stdout).Encode(response)
			return err
		}
		if ok {
			response.Status = "launched"
			response.SurfaceID = surface.Surface.ID
			response.Surface = &launchSurfaceEnvelope{Surface: launchSurfaceIdentity{
				ID:    surface.Surface.ID,
				AppID: surface.Surface.AppID,
				Title: surface.Surface.Title,
			}}
		} else {
			response.Status = "timed_out"
		}
	}
	return json.NewEncoder(stdout).Encode(response)
}

func applyCredential(cmd *exec.Cmd, uid uint32, gid uint32) error {
	currentUID := uint32(os.Geteuid())
	currentGID := uint32(os.Getegid())
	if uid == 0 && gid == 0 {
		return nil
	}
	if uid == 0 {
		uid = currentUID
	}
	if gid == 0 {
		gid = currentGID
	}
	if uid == currentUID && gid == currentGID {
		return nil
	}
	if currentUID != 0 {
		return fmt.Errorf("cannot launch as uid=%d gid=%d from uid=%d", uid, gid, currentUID)
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: uid, Gid: gid}}
	return nil
}

type launchResponse struct {
	LaunchID            string                 `json:"launch_id"`
	PID                 int                    `json:"pid"`
	SurfaceID           string                 `json:"surface_id,omitempty"`
	Status              string                 `json:"status"`
	SessionTokenPresent bool                   `json:"session_token_present"`
	AuditCorrelationID  string                 `json:"audit_correlation_id,omitempty"`
	OutputName          string                 `json:"output,omitempty"`
	Surface             *launchSurfaceEnvelope `json:"surface,omitempty"`
}

type launchSurfaceEnvelope struct {
	Surface launchSurfaceIdentity `json:"surface"`
}

type launchSurfaceIdentity struct {
	ID    string `json:"id"`
	AppID string `json:"app_id,omitempty"`
	Title string `json:"title,omitempty"`
}

type surfaceListResponse struct {
	Surfaces []trackedSurface `json:"surfaces"`
}

type trackedSurface struct {
	Surface struct {
		ID      string `json:"id"`
		AppID   string `json:"app_id"`
		Title   string `json:"title"`
		Visible bool   `json:"visible"`
	} `json:"surface"`
	Client struct {
		PID int `json:"pid"`
	} `json:"client"`
	Mapped    bool      `json:"mapped"`
	Visible   bool      `json:"visible"`
	UpdatedAt time.Time `json:"updated_at"`
}

func waitForSurface(rootPID int, startedAt time.Time, timeout time.Duration, done <-chan error) (trackedSurface, bool, error) {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		surface, ok := findSurfaceForPID(rootPID, startedAt)
		if ok {
			return surface, true, nil
		}
		select {
		case err := <-done:
			if err != nil {
				return trackedSurface{}, false, err
			}
			return trackedSurface{}, false, nil
		case <-ctx.Done():
			return trackedSurface{}, false, nil
		case <-ticker.C:
		}
	}
}

func findSurfaceForPID(rootPID int, startedAt time.Time) (trackedSurface, bool) {
	surfaces, err := listSurfacesFunc()
	if err != nil {
		return trackedSurface{}, false
	}
	for _, surface := range surfaces {
		if surface.Surface.ID == "" || surface.Client.PID <= 0 {
			continue
		}
		if !surface.Mapped && !surface.Visible && !surface.Surface.Visible {
			continue
		}
		if !surface.UpdatedAt.IsZero() && surface.UpdatedAt.Before(startedAt.Add(-500*time.Millisecond)) {
			continue
		}
		if surface.Client.PID == rootPID || processDescendsFrom(surface.Client.PID, rootPID) {
			return surface, true
		}
	}
	return trackedSurface{}, false
}

func listSurfaces() ([]trackedSurface, error) {
	path := strings.TrimSpace(os.Getenv("AGORA_DE_READBACK_COMPOSITORCTL"))
	if path == "" {
		path = defaultReadbackCompositorctl
	}
	if path == "" {
		return nil, errors.New("readback compositorctl is required")
	}
	output, err := exec.Command(path, "list-surfaces").Output()
	if err != nil {
		return nil, err
	}
	var response surfaceListResponse
	if err := json.Unmarshal(output, &response); err != nil {
		return nil, err
	}
	return response.Surfaces, nil
}

func processDescendsFrom(pid int, ancestor int) bool {
	for pid > 1 {
		parent, err := parentPID(pid)
		if err != nil || parent <= 0 {
			return false
		}
		if parent == ancestor {
			return true
		}
		pid = parent
	}
	return false
}

func parentPID(pid int) (int, error) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return 0, err
	}
	raw := string(data)
	endCommand := strings.LastIndex(raw, ") ")
	if endCommand < 0 || endCommand+2 >= len(raw) {
		return 0, fmt.Errorf("invalid proc stat for pid %d", pid)
	}
	fields := strings.Fields(raw[endCommand+2:])
	if len(fields) < 2 {
		return 0, fmt.Errorf("invalid proc stat for pid %d", pid)
	}
	return strconv.Atoi(fields[1])
}
