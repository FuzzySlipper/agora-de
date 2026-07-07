package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const defaultCompositorControlSocket = "/run/agent-os/compositor-control.sock"
const launchSurfacePollInterval = 50 * time.Millisecond

const (
	launchStatusLaunched                    = "launched"
	launchStatusLaunchedWithoutSurface      = "launched_without_surface"
	launchStatusSurfaceObservedAfterTimeout = "surface_observed_after_timeout"
	launchStatusTimedOutNoSurface           = "timed_out_no_surface"
	launchStatusReusedExistingWindow        = "reused_existing_window"
	launchStatusFailed                      = "failed"
)

const (
	methodListSurfaces         = "list_surfaces"
	methodListOutputs          = "list_outputs"
	methodCaptureOutput        = "capture_output"
	methodGetLayout            = "get_layout"
	methodSetLayoutMode        = "set_layout_mode"
	methodUpdateLayoutSettings = "update_layout_settings"
	methodFocusSurface         = "focus_surface"
	methodCloseSurface         = "close_surface"
	methodMoveResizeSurface    = "move_resize_surface"
	methodTileSurface          = "tile_surface"
	methodSetSurfaceFloating   = "set_surface_floating"
	methodAssignSurfaceZone    = "assign_surface_zone"
	methodPromoteSurface       = "promote_surface"
	methodMaximizeSurface      = "maximize_surface"
	methodMinimizeSurface      = "minimize_surface"
	methodFullscreenSurface    = "fullscreen_surface"
	methodActivateWorkspace    = "activate_workspace"
)

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
	pretty := false
	if args[0] == "--pretty" {
		pretty = true
		args = args[1:]
		if len(args) == 0 {
			usage(stderr)
			return errors.New("command is required")
		}
	}
	switch args[0] {
	case "launch":
		return runLaunch(args[1:], stdout)
	case "list-surfaces":
		return callAndPrint(methodListSurfaces, nil, stdout, pretty)
	case "layout":
		return runLayout(args[1:], stdout, pretty)
	case "output":
		return runOutput(args[1:], stdout, pretty)
	case "surface":
		return runSurface(args[1:], stdout, pretty)
	case "workspace":
		return runWorkspace(args[1:], stdout, pretty)
	default:
		usage(stderr)
		return fmt.Errorf("unsupported command %q", args[0])
	}
}

func usage(output io.Writer) {
	fmt.Fprintln(output, `Usage: compositorctl [--pretty] <command> [flags]

Commands:
  launch         Launch a native process from a structured argv vector
  layout         Read or change structured layout state
  list-surfaces  List tracked compositor surfaces
  output         List outputs or capture a physical output
  surface        Focus, close, or request layout actions for a tracked surface
  workspace      Request workspace actions`)
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
	rawURL := fs.String("url", "", "remote URL to open in a webview")
	rawPath := fs.String("path", "", "local HTML file to open in a webview")
	webviewTitle := fs.String("webview-title", "", "webview window title")
	webviewAppID := fs.String("app-id", "", "webview application id")
	expectedAppID := fs.String("expected-app-id", "", "expected compositor app id")
	webviewWidth := fs.Int("width", 1280, "webview width")
	webviewHeight := fs.Int("height", 800, "webview height")
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
	webviewLaunch := *rawURL != "" || *rawPath != ""
	if webviewLaunch {
		if len(argv) > 0 {
			return errors.New("--url/--path cannot be combined with --arg")
		}
		if *rawURL != "" && *rawPath != "" {
			return errors.New("only one of --url or --path may be provided")
		}
		built, err := buildWebviewArgv(webviewLaunchRequest{
			URL:           *rawURL,
			Path:          *rawPath,
			Title:         *webviewTitle,
			AppID:         *webviewAppID,
			ExpectedAppID: *expectedAppID,
			Width:         *webviewWidth,
			Height:        *webviewHeight,
		})
		if err != nil {
			return err
		}
		argv = built
	}
	if len(argv) == 0 {
		return errors.New("launch requires at least one --arg or --url/--path")
	}
	if !webviewLaunch && *sessionToken == "" {
		return errors.New("--session-token is required")
	}
	if !webviewLaunch && *auditCorrelationID == "" {
		return errors.New("--audit-correlation-id is required")
	}

	launchID := fmt.Sprintf("launch-%d", time.Now().UnixNano())
	startedAt := time.Now()
	var reusableIDs map[string]bool
	if *waitSurface {
		reusableIDs = reusableSurfaceIDs(launchSurfaceMatch{
			StartedAt:     startedAt,
			ExpectedAppID: *expectedAppID,
			ExpectedTitle: *webviewTitle,
		})
	}
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
		Status:              launchStatusLaunchedWithoutSurface,
		SessionTokenPresent: *sessionToken != "",
		AuditCorrelationID:  *auditCorrelationID,
		OutputName:          *outputName,
	}
	if *waitSurface {
		timeout := time.Duration(*waitTimeoutMs) * time.Millisecond
		observation, err := waitForSurface(launchSurfaceMatch{
			RootPID:       cmd.Process.Pid,
			StartedAt:     startedAt,
			ExpectedAppID: *expectedAppID,
			ExpectedTitle: *webviewTitle,
			ReusableIDs:   reusableIDs,
		}, timeout, done)
		if err != nil {
			response.Status = launchStatusFailed
			_ = json.NewEncoder(stdout).Encode(response)
			return err
		}
		response.Status = observation.Status
		if observation.Surface.Surface.ID != "" {
			response.SurfaceID = observation.Surface.Surface.ID
			response.Surface = &launchSurfaceEnvelope{Surface: launchSurfaceIdentity{
				ID:    observation.Surface.Surface.ID,
				AppID: observation.Surface.Surface.AppID,
				Title: observation.Surface.Surface.Title,
			}}
		}
	}
	return json.NewEncoder(stdout).Encode(response)
}

type webviewLaunchRequest struct {
	URL           string
	Path          string
	Title         string
	AppID         string
	ExpectedAppID string
	Width         int
	Height        int
}

func buildWebviewArgv(request webviewLaunchRequest) (repeatedFlag, error) {
	if request.URL == "" && request.Path == "" {
		return nil, errors.New("--url or --path is required")
	}
	if request.AppID == "" {
		return nil, errors.New("--app-id is required for webview launches")
	}
	if request.Title == "" {
		request.Title = request.AppID
	}
	if request.ExpectedAppID == "" {
		request.ExpectedAppID = request.AppID
	}
	if request.Width <= 0 {
		request.Width = 1280
	}
	if request.Height <= 0 {
		request.Height = 800
	}

	argv := repeatedFlag{webviewPython(), "-c", webviewPythonProgram}
	if request.URL != "" {
		argv = append(argv, "--url", request.URL)
	} else {
		argv = append(argv, "--path", request.Path)
	}
	argv = append(argv,
		"--title", request.Title,
		"--app-id", request.AppID,
		"--expected-app-id", request.ExpectedAppID,
		"--width", strconv.Itoa(request.Width),
		"--height", strconv.Itoa(request.Height),
	)
	return argv, nil
}

func webviewPython() string {
	if configured := strings.TrimSpace(os.Getenv("AGORA_DE_WEBVIEW_PYTHON")); configured != "" {
		return configured
	}
	return "/usr/bin/python3"
}

const webviewPythonProgram = `
import argparse
import json
import pathlib
import sys

parser = argparse.ArgumentParser()
parser.add_argument("--url", default="")
parser.add_argument("--path", default="")
parser.add_argument("--title", required=True)
parser.add_argument("--app-id", required=True)
parser.add_argument("--expected-app-id", required=True)
parser.add_argument("--width", type=int, default=1280)
parser.add_argument("--height", type=int, default=800)
args = parser.parse_args()

try:
    import gi
    gi.require_version("Gtk", "4.0")
    gi.require_version("WebKit", "6.0")
    from gi.repository import Gio, GLib, Gtk, WebKit
except Exception as exc:
    print(f"DEPENDENCY_MISSING GTK4/WebKit stack: {exc}", file=sys.stderr, flush=True)
    raise SystemExit(2)

class WebviewApp(Gtk.Application):
    def __init__(self):
        GLib.set_prgname(args.app_id)
        GLib.set_application_name(args.title)
        super().__init__(application_id=args.app_id, flags=Gio.ApplicationFlags.FLAGS_NONE)
        self.window = None

    def do_activate(self):
        self.window = Gtk.ApplicationWindow(application=self)
        self.window.set_title(args.title)
        self.window.set_default_size(args.width, args.height)
        webview = WebKit.WebView()
        if args.url:
            webview.load_uri(args.url)
        else:
            webview.load_uri(pathlib.Path(args.path).resolve().as_uri())
        self.window.set_child(webview)
        self.window.connect("destroy", lambda *_args: self.quit())
        self.window.present()
        print(json.dumps({"event": "shown", "appId": args.app_id, "expectedAppId": args.expected_app_id}), flush=True)

raise SystemExit(WebviewApp().run([]))
`

func runOutput(args []string, stdout io.Writer, pretty bool) error {
	if len(args) == 0 {
		return errors.New("output subcommand is required: list or capture")
	}
	switch args[0] {
	case "list":
		return callAndPrint(methodListOutputs, map[string]string{}, stdout, pretty)
	case "capture":
		fs := flag.NewFlagSet("output capture", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		name := fs.String("name", "", "logical output name")
		exportArtifact := fs.Bool("export", false, "write structured artifacts for captured surfaces")
		sessionID := fs.String("session", "", "session id for artifact export")
		sessionToken := fs.String("session-token", os.Getenv("AGORA_COMPOSITOR_SESSION_TOKEN"), "session token")
		auditID := fs.String("audit-correlation-id", "", "audit correlation id")
		evidenceClass := fs.String("evidence-class", "viewport_screenshot", "evidence class")
		seqID := fs.String("asha-command-sequence-id", "", "ASHA command sequence id")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *name == "" {
			return errors.New("--name is required")
		}
		req := captureOutputRequest{
			Name:                  *name,
			Export:                *exportArtifact,
			SessionID:             *sessionID,
			SessionToken:          *sessionToken,
			AuditCorrelationID:    *auditID,
			EvidenceClass:         *evidenceClass,
			ASHACommandSequenceID: *seqID,
		}
		return callAndPrint(methodCaptureOutput, req, stdout, pretty)
	default:
		return fmt.Errorf("unknown output subcommand %q", args[0])
	}
}

func runLayout(args []string, stdout io.Writer, pretty bool) error {
	if len(args) == 0 {
		return errors.New("layout subcommand is required: get, set-mode, or set-settings")
	}
	switch args[0] {
	case "get":
		return callAndPrint(methodGetLayout, nil, stdout, pretty)
	case "set-mode":
		fs := flag.NewFlagSet("layout set-mode", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		mode := fs.String("mode", "", "layout mode: freeform, zones, or columns")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *mode == "" {
			return errors.New("--mode is required")
		}
		return callAndPrint(methodSetLayoutMode, setLayoutModeRequest{Mode: *mode}, stdout, pretty)
	case "set-settings":
		request, err := buildUpdateLayoutSettingsRequest(args[1:])
		if err != nil {
			return err
		}
		return callAndPrint(methodUpdateLayoutSettings, request, stdout, pretty)
	default:
		return fmt.Errorf("unknown layout subcommand %q", args[0])
	}
}

func buildUpdateLayoutSettingsRequest(args []string) (updateLayoutSettingsRequest, error) {
	fs := flag.NewFlagSet("layout set-settings", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	rule := fs.String("rule", "", "layout rule: master_stack, zones, or dwindle")
	mode := fs.String("mode", "", "layout mode: freeform, zones, or columns")
	outerHorizontal := fs.Int("outer-horizontal", 0, "outer horizontal gap")
	outerVertical := fs.Int("outer-vertical", 0, "outer vertical gap")
	innerHorizontal := fs.Int("inner-horizontal", 0, "inner horizontal gap")
	innerVertical := fs.Int("inner-vertical", 0, "inner vertical gap")
	masterCount := fs.Int("master-count", 0, "master area surface count")
	masterRatio := fs.Float64("master-ratio", 0, "master area ratio")
	smartGaps := fs.Bool("smart-gaps", false, "enable smart gaps")
	if err := fs.Parse(args); err != nil {
		return updateLayoutSettingsRequest{}, err
	}
	seen := map[string]bool{}
	fs.Visit(func(flag *flag.Flag) {
		seen[flag.Name] = true
	})
	if len(seen) == 0 {
		return updateLayoutSettingsRequest{}, errors.New("at least one layout setting flag is required")
	}
	request := updateLayoutSettingsRequest{}
	if seen["rule"] {
		value := strings.TrimSpace(*rule)
		if value == "" {
			return updateLayoutSettingsRequest{}, errors.New("--rule cannot be empty")
		}
		request.Rule = &value
	}
	if seen["mode"] {
		value := strings.TrimSpace(*mode)
		if value == "" {
			return updateLayoutSettingsRequest{}, errors.New("--mode cannot be empty")
		}
		request.Mode = &value
	}
	if seen["outer-horizontal"] || seen["outer-vertical"] || seen["inner-horizontal"] || seen["inner-vertical"] {
		request.Gaps = &layoutGaps{
			OuterHorizontal: *outerHorizontal,
			OuterVertical:   *outerVertical,
			InnerHorizontal: *innerHorizontal,
			InnerVertical:   *innerVertical,
		}
	}
	if seen["master-count"] {
		request.MasterCount = masterCount
	}
	if seen["master-ratio"] {
		request.MasterRatio = masterRatio
	}
	if seen["smart-gaps"] {
		request.SmartGaps = smartGaps
	}
	return request, nil
}

func runSurface(args []string, stdout io.Writer, pretty bool) error {
	if len(args) == 0 {
		return errors.New("surface subcommand is required")
	}
	switch args[0] {
	case "focus":
		req, err := buildSurfaceRequest("surface focus", args[1:])
		if err != nil {
			return err
		}
		return callAndPrint(methodFocusSurface, req, stdout, pretty)
	case "close":
		req, err := buildSurfaceRequest("surface close", args[1:])
		if err != nil {
			return err
		}
		return callAndPrint(methodCloseSurface, req, stdout, pretty)
	case "move-resize":
		req, err := buildMoveResizeSurfaceRequest(args[1:])
		if err != nil {
			return err
		}
		return callAndPrint(methodMoveResizeSurface, req, stdout, pretty)
	case "tile":
		req, err := buildZoneSurfaceRequest("surface tile", args[1:])
		if err != nil {
			return err
		}
		return callAndPrint(methodTileSurface, req, stdout, pretty)
	case "set-floating":
		req, err := buildFloatingSurfaceRequest(args[1:])
		if err != nil {
			return err
		}
		return callAndPrint(methodSetSurfaceFloating, req, stdout, pretty)
	case "assign-zone":
		req, err := buildZoneSurfaceRequest("surface assign-zone", args[1:])
		if err != nil {
			return err
		}
		return callAndPrint(methodAssignSurfaceZone, req, stdout, pretty)
	case "promote":
		req, err := buildSurfaceRequest("surface promote", args[1:])
		if err != nil {
			return err
		}
		return callAndPrint(methodPromoteSurface, surfaceLayoutRequest{SurfaceID: req.SurfaceID, WaitTimeoutMs: req.WaitTimeoutMs}, stdout, pretty)
	case "maximize":
		req, err := buildEnabledSurfaceRequest("surface maximize", args[1:])
		if err != nil {
			return err
		}
		return callAndPrint(methodMaximizeSurface, req, stdout, pretty)
	case "minimize":
		req, err := buildEnabledSurfaceRequest("surface minimize", args[1:])
		if err != nil {
			return err
		}
		return callAndPrint(methodMinimizeSurface, req, stdout, pretty)
	case "fullscreen":
		req, err := buildEnabledSurfaceRequest("surface fullscreen", args[1:])
		if err != nil {
			return err
		}
		return callAndPrint(methodFullscreenSurface, req, stdout, pretty)
	default:
		return fmt.Errorf("unknown surface subcommand %q", args[0])
	}
}

func runWorkspace(args []string, stdout io.Writer, pretty bool) error {
	if len(args) == 0 {
		return errors.New("workspace subcommand is required: activate")
	}
	switch args[0] {
	case "activate":
		fs := flag.NewFlagSet("workspace activate", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		workspaceID := fs.String("workspace", "", "workspace id")
		timeoutMs := fs.Int("timeout-ms", 2000, "acknowledgement timeout in milliseconds")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *workspaceID == "" {
			return errors.New("--workspace is required")
		}
		return callAndPrint(methodActivateWorkspace, workspaceRequest{WorkspaceID: *workspaceID, WaitTimeoutMs: *timeoutMs}, stdout, pretty)
	default:
		return fmt.Errorf("unknown workspace subcommand %q", args[0])
	}
}

func buildSurfaceRequest(name string, args []string) (surfaceRequest, error) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	surfaceID := fs.String("surface", "", "surface id")
	timeoutMs := fs.Int("timeout-ms", 2000, "acknowledgement timeout in milliseconds")
	if err := fs.Parse(args); err != nil {
		return surfaceRequest{}, err
	}
	if *surfaceID == "" {
		return surfaceRequest{}, errors.New("--surface is required")
	}
	return surfaceRequest{SurfaceID: *surfaceID, WaitTimeoutMs: *timeoutMs}, nil
}

func buildMoveResizeSurfaceRequest(args []string) (surfaceLayoutRequest, error) {
	fs := flag.NewFlagSet("surface move-resize", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	surfaceID := fs.String("surface", "", "surface id")
	x := fs.Int("x", 0, "surface x coordinate")
	y := fs.Int("y", 0, "surface y coordinate")
	width := fs.Int("width", 0, "surface width")
	height := fs.Int("height", 0, "surface height")
	timeoutMs := fs.Int("timeout-ms", 2000, "acknowledgement timeout in milliseconds")
	if err := fs.Parse(args); err != nil {
		return surfaceLayoutRequest{}, err
	}
	if *surfaceID == "" {
		return surfaceLayoutRequest{}, errors.New("--surface is required")
	}
	if *width <= 0 || *height <= 0 {
		return surfaceLayoutRequest{}, errors.New("--width and --height must be positive")
	}
	return surfaceLayoutRequest{
		SurfaceID:     *surfaceID,
		Geometry:      &surfaceGeometry{X: *x, Y: *y, Width: *width, Height: *height},
		WaitTimeoutMs: *timeoutMs,
	}, nil
}

func buildZoneSurfaceRequest(name string, args []string) (surfaceLayoutRequest, error) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	surfaceID := fs.String("surface", "", "surface id")
	workspaceID := fs.String("workspace", "", "workspace id")
	zoneID := fs.String("zone", "", "zone id")
	x := fs.Int("x", 0, "planned surface x coordinate")
	y := fs.Int("y", 0, "planned surface y coordinate")
	width := fs.Int("width", 0, "planned surface width")
	height := fs.Int("height", 0, "planned surface height")
	timeoutMs := fs.Int("timeout-ms", 2000, "acknowledgement timeout in milliseconds")
	if err := fs.Parse(args); err != nil {
		return surfaceLayoutRequest{}, err
	}
	if *surfaceID == "" {
		return surfaceLayoutRequest{}, errors.New("--surface is required")
	}
	if *zoneID == "" {
		return surfaceLayoutRequest{}, errors.New("--zone is required")
	}
	req := surfaceLayoutRequest{SurfaceID: *surfaceID, WorkspaceID: *workspaceID, ZoneID: *zoneID, WaitTimeoutMs: *timeoutMs}
	if *x != 0 || *y != 0 || *width != 0 || *height != 0 {
		if *width <= 0 || *height <= 0 {
			return surfaceLayoutRequest{}, errors.New("--width and --height must be positive when planner geometry is supplied")
		}
		req.Geometry = &surfaceGeometry{X: *x, Y: *y, Width: *width, Height: *height}
	}
	return req, nil
}

func buildFloatingSurfaceRequest(args []string) (surfaceLayoutRequest, error) {
	req, err := buildEnabledSurfaceRequest("surface set-floating", args)
	if err != nil {
		return surfaceLayoutRequest{}, err
	}
	req.Floating = req.Enabled
	req.Enabled = nil
	return req, nil
}

func buildEnabledSurfaceRequest(name string, args []string) (surfaceLayoutRequest, error) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	surfaceID := fs.String("surface", "", "surface id")
	enabled := fs.Bool("enabled", true, "whether the state should be enabled")
	timeoutMs := fs.Int("timeout-ms", 2000, "acknowledgement timeout in milliseconds")
	if err := fs.Parse(args); err != nil {
		return surfaceLayoutRequest{}, err
	}
	if *surfaceID == "" {
		return surfaceLayoutRequest{}, errors.New("--surface is required")
	}
	return surfaceLayoutRequest{SurfaceID: *surfaceID, Enabled: enabled, WaitTimeoutMs: *timeoutMs}, nil
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

type launchSurfaceMatch struct {
	RootPID       int
	StartedAt     time.Time
	ExpectedAppID string
	ExpectedTitle string
	ReusableIDs   map[string]bool
}

type launchSurfaceObservation struct {
	Surface trackedSurface
	Status  string
}

func waitForSurface(match launchSurfaceMatch, timeout time.Duration, done <-chan error) (launchSurfaceObservation, error) {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	grace := timeout / 2
	if grace > 3*time.Second {
		grace = 3 * time.Second
	}
	if grace < 500*time.Millisecond {
		grace = 500 * time.Millisecond
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout+grace)
	defer cancel()
	ticker := time.NewTicker(launchSurfacePollInterval)
	defer ticker.Stop()
	primaryDeadline := time.Now().Add(timeout)
	processDone := false

	for {
		surface, status, ok := findSurfaceForLaunch(match, time.Now().After(primaryDeadline))
		if ok {
			return launchSurfaceObservation{Surface: surface, Status: status}, nil
		}
		select {
		case err := <-done:
			if err != nil {
				return launchSurfaceObservation{Status: launchStatusFailed}, err
			}
			processDone = true
		case <-ctx.Done():
			if processDone {
				return launchSurfaceObservation{Status: launchStatusTimedOutNoSurface}, nil
			}
			return launchSurfaceObservation{Status: launchStatusTimedOutNoSurface}, nil
		case <-ticker.C:
		}
	}
}

func reusableSurfaceIDs(match launchSurfaceMatch) map[string]bool {
	if match.ExpectedAppID == "" && match.ExpectedTitle == "" {
		return nil
	}
	surfaces, err := listSurfacesFunc()
	if err != nil {
		return nil
	}
	ids := map[string]bool{}
	for _, surface := range surfaces {
		if surface.Surface.ID == "" || !surfaceVisible(surface) {
			continue
		}
		if launchHintMatches(surface, match) {
			ids[surface.Surface.ID] = true
		}
	}
	if len(ids) == 0 {
		return nil
	}
	return ids
}

func findSurfaceForLaunch(match launchSurfaceMatch, afterPrimaryTimeout bool) (trackedSurface, string, bool) {
	surfaces, err := listSurfacesFunc()
	if err != nil {
		return trackedSurface{}, "", false
	}
	for _, surface := range surfaces {
		if surface.Surface.ID == "" {
			continue
		}
		if !surfaceVisible(surface) {
			continue
		}
		if match.ReusableIDs[surface.Surface.ID] && launchHintMatches(surface, match) {
			return surface, launchStatusReusedExistingWindow, true
		}
		recent := surface.UpdatedAt.IsZero() || !surface.UpdatedAt.Before(match.StartedAt.Add(-500*time.Millisecond))
		if !recent {
			continue
		}
		status := launchStatusLaunched
		if afterPrimaryTimeout {
			status = launchStatusSurfaceObservedAfterTimeout
		}
		if surface.Client.PID > 0 && (surface.Client.PID == match.RootPID || processDescendsFrom(surface.Client.PID, match.RootPID)) {
			return surface, status, true
		}
		if launchHintMatches(surface, match) {
			return surface, status, true
		}
	}
	return trackedSurface{}, "", false
}

func surfaceVisible(surface trackedSurface) bool {
	return surface.Mapped || surface.Visible || surface.Surface.Visible
}

func launchHintMatches(surface trackedSurface, match launchSurfaceMatch) bool {
	if match.ExpectedAppID != "" && surface.Surface.AppID == match.ExpectedAppID {
		return true
	}
	return match.ExpectedTitle != "" && surface.Surface.Title == match.ExpectedTitle
}

func listSurfaces() ([]trackedSurface, error) {
	output, err := callCompositorControl(methodListSurfaces, nil)
	if err != nil {
		return nil, err
	}
	var response surfaceListResponse
	if err := json.Unmarshal(output, &response); err != nil {
		return nil, err
	}
	return response.Surfaces, nil
}

type controlRequest struct {
	Method string          `json:"method"`
	Body   json.RawMessage `json:"body"`
}

type controlResponse struct {
	OK           bool            `json:"ok"`
	Body         json.RawMessage `json:"body,omitempty"`
	ErrorClass   string          `json:"error_class,omitempty"`
	ErrorMessage string          `json:"error_message,omitempty"`
}

type captureOutputRequest struct {
	Name                  string `json:"name"`
	Export                bool   `json:"export,omitempty"`
	SessionID             string `json:"session_id,omitempty"`
	SessionToken          string `json:"session_token,omitempty"`
	AuditCorrelationID    string `json:"audit_correlation_id,omitempty"`
	EvidenceClass         string `json:"evidence_class,omitempty"`
	ASHACommandSequenceID string `json:"asha_command_sequence_id,omitempty"`
}

type setLayoutModeRequest struct {
	Mode string `json:"mode"`
}

type updateLayoutSettingsRequest struct {
	Rule        *string     `json:"rule,omitempty"`
	Mode        *string     `json:"mode,omitempty"`
	Gaps        *layoutGaps `json:"gaps,omitempty"`
	MasterCount *int        `json:"master_count,omitempty"`
	MasterRatio *float64    `json:"master_ratio,omitempty"`
	SmartGaps   *bool       `json:"smart_gaps,omitempty"`
}

type layoutGaps struct {
	OuterHorizontal int `json:"outer_horizontal"`
	OuterVertical   int `json:"outer_vertical"`
	InnerHorizontal int `json:"inner_horizontal"`
	InnerVertical   int `json:"inner_vertical"`
}

type surfaceRequest struct {
	SurfaceID     string `json:"surface_id"`
	WaitTimeoutMs int    `json:"wait_timeout_ms,omitempty"`
}

type surfaceGeometry struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

type surfaceLayoutRequest struct {
	SurfaceID     string           `json:"surface_id"`
	Geometry      *surfaceGeometry `json:"geometry,omitempty"`
	WorkspaceID   string           `json:"workspace_id,omitempty"`
	ZoneID        string           `json:"zone_id,omitempty"`
	Floating      *bool            `json:"floating,omitempty"`
	Enabled       *bool            `json:"enabled,omitempty"`
	WaitTimeoutMs int              `json:"wait_timeout_ms,omitempty"`
}

type workspaceRequest struct {
	WorkspaceID   string `json:"workspace_id"`
	WaitTimeoutMs int    `json:"wait_timeout_ms,omitempty"`
}

func callAndPrint(method string, body any, stdout io.Writer, pretty bool) error {
	response, err := callCompositorControl(method, body)
	if err != nil {
		return err
	}
	return printJSON(response, stdout, pretty)
}

func callCompositorControl(method string, body any) (json.RawMessage, error) {
	socketPath := compositorControlSocket()
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return nil, fmt.Errorf("connect %s: %w", socketPath, err)
	}
	defer conn.Close()

	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	request := controlRequest{Method: method, Body: bodyJSON}
	if err := json.NewEncoder(conn).Encode(request); err != nil {
		return nil, fmt.Errorf("send: %w", err)
	}

	var response controlResponse
	if err := json.NewDecoder(conn).Decode(&response); err != nil {
		return nil, fmt.Errorf("recv: %w", err)
	}
	if !response.OK {
		if response.ErrorClass != "" {
			return nil, fmt.Errorf("server[%s]: %s", response.ErrorClass, response.ErrorMessage)
		}
		return nil, fmt.Errorf("server: %s", string(response.Body))
	}
	if len(response.Body) == 0 {
		return json.RawMessage(`{}`), nil
	}
	return response.Body, nil
}

func compositorControlSocket() string {
	for _, key := range []string{"AGORA_DE_COMPOSITOR_CONTROL_SOCKET", "AGORA_DE_COMPOSITORCTL_SOCKET"} {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	return defaultCompositorControlSocket
}

func printJSON(data json.RawMessage, stdout io.Writer, pretty bool) error {
	if pretty {
		var decoded any
		if err := json.Unmarshal(data, &decoded); err != nil {
			return err
		}
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(decoded)
	}
	_, err := stdout.Write(append(data, '\n'))
	return err
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
