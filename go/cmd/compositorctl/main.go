package main

import (
	"bytes"
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

// agentLaunchPolicy selects how much ceremony native launch requires.
// "open" (default): agents pop native windows with no session token or audit
// id — real-app GUI testing is the priority, and the bridge does not enforce
// those fields anyway. "governed": restore the session-token + audit-correlation
// requirement as a future hook for agora-os governance handoff. Set via
// AGORA_DE_AGENT_LAUNCH_POLICY.
func agentLaunchPolicy() string {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("AGORA_DE_AGENT_LAUNCH_POLICY"))) {
	case "governed":
		return "governed"
	default:
		return "open"
	}
}

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
	methodSetSurfaceOrder      = "set_surface_order"
	methodCloseSurface         = "close_surface"
	methodMoveResizeSurface    = "move_resize_surface"
	methodTileSurface          = "tile_surface"
	methodSetSurfaceFloating   = "set_surface_floating"
	methodAssignSurfaceZone    = "assign_surface_zone"
	methodPromoteSurface       = "promote_surface"
	methodMoveSurface          = "move_surface"
	methodSwapMasterSurface    = "swap_master_surface"
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
	case "input":
		return runInput(args[1:], stdout, pretty)
	default:
		usage(stderr)
		return fmt.Errorf("unsupported command %q", args[0])
	}
}

func usage(output io.Writer) {
	fmt.Fprintln(output, `Usage: agora-de-compositorctl [--pretty] <command> [flags]

Commands:
  launch         Launch a native process from a structured argv vector
  layout         Read or change structured layout state
  list-surfaces  List tracked compositor surfaces
  output         List outputs or capture a physical output
  surface        Focus, close, or request layout actions for a tracked surface
  input          Inject widget input (pointer move/click) into a tracked surface
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
	sessionToken := fs.String("session-token", "", "session token (optional under open launch policy; required under governed)")
	auditCorrelationID := fs.String("audit-correlation-id", "", "audit correlation id (optional under open launch policy; required under governed)")
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
	if !webviewLaunch && agentLaunchPolicy() == "governed" {
		if *sessionToken == "" {
			return errors.New("--session-token is required under governed launch policy")
		}
		if *auditCorrelationID == "" {
			return errors.New("--audit-correlation-id is required under governed launch policy")
		}
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
		return errors.New("layout subcommand is required: get, set-mode, set-settings, cycle-mode, cycle-rule, save, restore, list, or delete")
	}
	switch args[0] {
	case "get":
		return callAndPrint(methodGetLayout, nil, stdout, pretty)
	case "cycle-mode":
		info, err := fetchLayout()
		if err != nil {
			return err
		}
		current := info.Mode
		if current == "" {
			current = info.Settings.Mode
		}
		return callAndPrint(methodSetLayoutMode, setLayoutModeRequest{Mode: cycleString([]string{"freeform", "zones", "columns"}, current)}, stdout, pretty)
	case "cycle-rule":
		info, err := fetchLayout()
		if err != nil {
			return err
		}
		rule := cycleString([]string{"master_stack", "zones", "dwindle"}, info.Settings.Rule)
		return callAndPrint(methodUpdateLayoutSettings, updateLayoutSettingsRequest{Rule: &rule}, stdout, pretty)
	case "save":
		name, err := requireSessionName("save", args[1:])
		if err != nil {
			return err
		}
		return saveLayoutSession(name)
	case "restore":
		name, err := requireSessionName("restore", args[1:])
		if err != nil {
			return err
		}
		return restoreLayoutSession(name)
	case "list":
		return listLayoutSessions()
	case "delete":
		name, err := requireSessionName("delete", args[1:])
		if err != nil {
			return err
		}
		return deleteLayoutSession(name)
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
		return errors.New("surface subcommand is required: focus, close, move-resize, tile, set-floating, assign-zone, promote, move, swap-master, maximize, minimize, fullscreen")
	}
	switch args[0] {
	case "focus":
		req, err := buildSurfaceRequest("surface focus", args[1:])
		if err != nil {
			return err
		}
		return callAndPrint(methodFocusSurface, req, stdout, pretty)
	case "focus-next", "focus-prev":
		return runSurfaceFocusCycle(args[0], stdout, pretty)
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
	case "move":
		return runSurfaceMove(args[1:], stdout, pretty)
	case "swap-master":
		req, err := buildSurfaceRequest("surface swap-master", args[1:])
		if err != nil {
			return err
		}
		return callAndPrint(methodSwapMasterSurface, surfaceLayoutRequest{SurfaceID: req.SurfaceID, WaitTimeoutMs: req.WaitTimeoutMs}, stdout, pretty)
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
		outputID := fs.String("output", "", "owning output id")
		timeoutMs := fs.Int("timeout-ms", 2000, "acknowledgement timeout in milliseconds")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *workspaceID == "" {
			return errors.New("--workspace is required")
		}
		return callAndPrint(methodActivateWorkspace, workspaceRequest{WorkspaceID: *workspaceID, OutputID: *outputID, WaitTimeoutMs: *timeoutMs}, stdout, pretty)
	default:
		return fmt.Errorf("unknown workspace subcommand %q", args[0])
	}
}

const defaultInputHelper = "agora-de-wayland-input"

func runInput(args []string, stdout io.Writer, _ bool) error {
	if len(args) == 0 {
		return errors.New("input subcommand is required: pointer|keyboard")
	}
	switch device := args[0]; device {
	case "pointer":
		return runInputPointer(args[1:], stdout)
	case "keyboard":
		return runInputKeyboard(args[1:], stdout)
	default:
		return fmt.Errorf("unknown input device %q (pointer or keyboard)", device)
	}
}

func runInputPointer(args []string, stdout io.Writer) error {
	if len(args) < 1 {
		return errors.New("pointer action is required: move or click")
	}
	action := args[0]
	switch action {
	case "move", "click":
	default:
		return fmt.Errorf("unknown pointer action %q (move or click)", action)
	}

	fs := flag.NewFlagSet("input pointer", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	surfaceID := fs.String("surface", "", "tracked surface id to target (required)")
	x := fs.Int("x", 0, "pointer x coordinate, output-relative")
	y := fs.Int("y", 0, "pointer y coordinate, output-relative")
	button := fs.Uint("button", 0x110, "button code for click (default 0x110 BTN_LEFT)")
	outputW := fs.Int("output-w", 0, "output width for absolute motion (default: auto from output list)")
	outputH := fs.Int("output-h", 0, "output height for absolute motion (default: auto from output list)")
	helperPath := fs.String("helper", "", "path to agora-de-wayland-input (default: resolved from PATH/common paths)")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	target, err := verifyInputSurface(*surfaceID)
	if err != nil {
		return err
	}

	if *outputW <= 0 || *outputH <= 0 {
		if w, h, ok := outputExtents(target.Surface.OutputID); ok {
			*outputW, *outputH = w, h
		}
	}
	if *outputW <= 0 {
		*outputW = 2560
	}
	if *outputH <= 0 {
		*outputH = 1440
	}

	resolved, err := resolveInputHelper(*helperPath)
	if err != nil {
		return err
	}

	result := inputResult{
		Device:       "pointer",
		Action:       action,
		SurfaceID:    *surfaceID,
		X:            *x,
		Y:            *y,
		Button:       *button,
		OutputW:      *outputW,
		OutputH:      *outputH,
		SurfaceAppID: target.Surface.AppID,
	}
	helperOut, helperErr := execInputHelper(resolved, action, *x, *y, *button, *outputW, *outputH)
	result.Helper = helperOut
	if helperErr != nil {
		result.Ok = false
		result.Error = helperErr.Error()
		_ = json.NewEncoder(stdout).Encode(result)
		return fmt.Errorf("input injection failed: %w", helperErr)
	}
	result.Ok = true
	return json.NewEncoder(stdout).Encode(result)
}

func runInputKeyboard(args []string, stdout io.Writer) error {
	if len(args) < 1 {
		return errors.New("keyboard action is required: type or key")
	}
	action := args[0]
	switch action {
	case "type", "key":
	default:
		return fmt.Errorf("unknown keyboard action %q (type or key)", action)
	}
	fs := flag.NewFlagSet("input keyboard", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	surfaceID := fs.String("surface", "", "tracked surface id to target (required)")
	text := fs.String("text", "", "text to type (for action type)")
	key := fs.String("key", "", "xkb keysym name to press, e.g. Return, Escape, Tab, space (for action key)")
	method := fs.String("method", "auto", "type method: auto (input-method then virtual-keyboard), input-method (text-input-v3 clients like Chromium), virtual-keyboard (native wl_keyboard clients)")
	wtypePath := fs.String("wtype", "", "path to wtype (virtual-keyboard engine; default: resolved)")
	helperPath := fs.String("helper", "", "path to agora-de-wayland-input (input-method+pointer engine; default: resolved)")
	timeoutMs := fs.Int("timeout-ms", 4000, "input-method activate wait timeout in milliseconds")
	clickX := fs.Int("click-x", 0, "input-method: x to click to focus the text-input field (default: output center)")
	clickY := fs.Int("click-y", 0, "input-method: y to click to focus the text-input field (default: output center)")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if *surfaceID == "" {
		return errors.New("--surface is required")
	}
	if action == "type" && *text == "" {
		return errors.New("--text is required for type")
	}
	if action == "key" && *key == "" {
		return errors.New("--key is required for key")
	}
	target, err := verifyInputSurface(*surfaceID)
	if err != nil {
		return err
	}
	result := inputResult{
		Device:       "keyboard",
		Action:       action,
		SurfaceID:    *surfaceID,
		SurfaceAppID: target.Surface.AppID,
	}

	// `key` (Return/Tab/...) is a raw keyboard event: virtual-keyboard (wtype) only.
	if action == "key" {
		*method = "virtual-keyboard"
	}

	switch *method {
	case "input-method", "auto":
		cx, cy := *clickX, *clickY
		if cx <= 0 || cy <= 0 {
			if w, h, ok := outputExtents(target.Surface.OutputID); ok {
				cx, cy = w/2, h/2
			} else {
				cx, cy = 1280, 720
			}
		}
		ok, helperOut, helperErr := runInputMethodHelper(*helperPath, *surfaceID, cx, cy, *text, *timeoutMs)
		if ok {
			result.Ok = true
			result.Helper = helperOut
			return json.NewEncoder(stdout).Encode(result)
		}
		if *method == "input-method" {
			result.Ok = false
			result.Error = helperErr
			_ = json.NewEncoder(stdout).Encode(result)
			return fmt.Errorf("input-method keyboard injection failed: %s", helperErr)
		}
		result.Helper = "input-method: " + helperErr
	}
	// virtual-keyboard (wtype) path.
	_ = focusSurface(*surfaceID)
	time.Sleep(300 * time.Millisecond)
	resolved, err := resolveWtype(*wtypePath)
	if err != nil {
		result.Ok = false
		result.Error = err.Error()
		_ = json.NewEncoder(stdout).Encode(result)
		return err
	}
	var wargs []string
	if action == "key" {
		wargs = []string{"-k", *key}
	} else {
		wargs = []string{*text}
	}
	cmd := exec.Command(resolved, wargs...)
	cmd.Env = os.Environ()
	var out, errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	runErr := cmd.Run()
	if runErr != nil {
		result.Ok = false
		result.Error = strings.TrimSpace(errBuf.String() + " " + out.String())
		_ = json.NewEncoder(stdout).Encode(result)
		return fmt.Errorf("keyboard injection failed: %w", runErr)
	}
	result.Ok = true
	if result.Helper == "" {
		result.Helper = resolved
	} else {
		result.Helper = resolved + " (" + result.Helper + ")"
	}
	return json.NewEncoder(stdout).Encode(result)
}

// runInputMethodHelper commits text into a focused text-input-v3 client (e.g.
// Chromium) via the owned agora-de-wayland-input input-method path. The input
// method must be bound BEFORE the text-input field is focused (enabled), so this
// starts the helper (which binds and waits for activate), then clicks the field
// to trigger enable, then waits for the helper to commit. Returns ok=false if no
// text-input-v3 client activates (used by the auto method to fall through).
func runInputMethodHelper(explicit, surfaceID string, clickX, clickY int, text string, timeoutMs int) (bool, string, string) {
	resolved, err := resolveInputHelper(explicit)
	if err != nil {
		return false, "", err.Error()
	}
	helper := exec.Command(resolved, "input-method", "--text", text, "--timeout-ms", strconv.Itoa(timeoutMs))
	helper.Env = os.Environ()
	var hOut, hErr bytes.Buffer
	helper.Stdout = &hOut
	helper.Stderr = &hErr
	if err := helper.Start(); err != nil {
		return false, "", err.Error()
	}
	// give the helper time to bind zwp_input_method_v1, then click the field to
	// trigger the text-input-v3 enable -> activate.
	time.Sleep(500 * time.Millisecond)
	_ = focusSurface(surfaceID)
	time.Sleep(300 * time.Millisecond)
	_, _ = execInputHelper(resolved, "click", clickX, clickY, 0x110, 2560, 1440)
	waitErr := helper.Wait()
	if waitErr != nil {
		return false, "", strings.TrimSpace(hErr.String() + " " + hOut.String())
	}
	return true, strings.TrimSpace(hOut.String()), ""
}
func verifyInputSurface(surfaceID string) (*trackedSurface, error) {
	if surfaceID == "" {
		return nil, errors.New("--surface is required")
	}
	surfaces, err := listSurfacesFunc()
	if err != nil {
		return nil, fmt.Errorf("verify surface: %w", err)
	}
	for i := range surfaces {
		if surfaces[i].Surface.ID == surfaceID {
			if !surfaces[i].InputInjectable {
				return nil, fmt.Errorf("surface %q (%s) is not input-injectable (kind=%q)",
					surfaceID, surfaces[i].Surface.AppID, surfaces[i].Surface.SurfaceKind)
			}
			return &surfaces[i], nil
		}
	}
	return nil, fmt.Errorf("surface %q is not tracked; input targets must be tracked surfaces", surfaceID)
}

func focusSurface(surfaceID string) error {
	_, err := callCompositorControl(methodFocusSurface, surfaceRequest{SurfaceID: surfaceID, WaitTimeoutMs: 2000})
	return err
}

func resolveWtype(explicit string) (string, error) {
	if path := strings.TrimSpace(explicit); path != "" {
		return path, nil
	}
	if path := strings.TrimSpace(os.Getenv("AGORA_DE_WTYPE")); path != "" {
		return path, nil
	}
	if path, err := exec.LookPath("wtype"); err == nil {
		return path, nil
	}
	return "", fmt.Errorf("wtype not found; install wtype or set --wtype / AGORA_DE_WTYPE")
}

type inputResult struct {
	Ok           bool   `json:"ok"`
	Device       string `json:"device"`
	Action       string `json:"action"`
	SurfaceID    string `json:"surface_id"`
	SurfaceAppID string `json:"surface_app_id,omitempty"`
	X            int    `json:"x,omitempty"`
	Y            int    `json:"y,omitempty"`
	Button       uint   `json:"button,omitempty"`
	OutputW      int    `json:"output_w,omitempty"`
	OutputH      int    `json:"output_h,omitempty"`
	Helper       string `json:"helper,omitempty"`
	Error        string `json:"error,omitempty"`
}

func outputExtents(preferredOutput string) (int, int, bool) {
	raw, err := callCompositorControl(methodListOutputs, map[string]string{})
	if err != nil {
		return 0, 0, false
	}
	var resp struct {
		Outputs []struct {
			Name   string `json:"name"`
			Width  int    `json:"width"`
			Height int    `json:"height"`
		} `json:"outputs"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return 0, 0, false
	}
	for _, o := range resp.Outputs {
		if preferredOutput != "" && o.Name == preferredOutput && o.Width > 0 && o.Height > 0 {
			return o.Width, o.Height, true
		}
	}
	for _, o := range resp.Outputs {
		if o.Width > 0 && o.Height > 0 {
			return o.Width, o.Height, true
		}
	}
	return 0, 0, false
}

func resolveInputHelper(explicit string) (string, error) {
	if path := strings.TrimSpace(explicit); path != "" {
		return path, nil
	}
	if path := strings.TrimSpace(os.Getenv("AGORA_DE_WAYLAND_INPUT")); path != "" {
		return path, nil
	}
	if path, err := exec.LookPath(defaultInputHelper); err == nil {
		return path, nil
	}
	for _, candidate := range []string{
		os.Getenv("HOME") + "/.local/bin/" + defaultInputHelper,
		"/usr/local/bin/" + defaultInputHelper,
	} {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("input helper %q not found; build chrome/wayland-input or set --helper / AGORA_DE_WAYLAND_INPUT", defaultInputHelper)
}

func execInputHelper(path, action string, x, y int, button uint, outputW, outputH int) (string, error) {
	cmd := exec.Command(path,
		"pointer",
		"--action", action,
		"--x", strconv.Itoa(x),
		"--y", strconv.Itoa(y),
		"--output-w", strconv.Itoa(outputW),
		"--output-h", strconv.Itoa(outputH),
	)
	if action == "click" {
		cmd.Args = append(cmd.Args, "--button", strconv.FormatUint(uint64(button), 10))
	}
	cmd.Env = os.Environ()
	var out, errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		tail := strings.TrimSpace(errBuf.String())
		if tail == "" {
			tail = strings.TrimSpace(out.String())
		}
		return "", fmt.Errorf("%s: %s", path, firstLine(tail))
	}
	return strings.TrimSpace(out.String()), nil
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
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
	resolved, err := resolveSurfaceID(*surfaceID)
	if err != nil {
		return surfaceRequest{}, err
	}
	return surfaceRequest{SurfaceID: resolved, WaitTimeoutMs: *timeoutMs}, nil
}

// resolveSurfaceID turns the literal token "focused" into the currently-focused
// surface id (via get_layout); any other value passes through unchanged. This
// lets keybindings drive the focused window without knowing its id.
func resolveSurfaceID(surfaceID string) (string, error) {
	if strings.TrimSpace(surfaceID) != "focused" {
		return surfaceID, nil
	}
	focused, err := focusedSurfaceID()
	if err != nil {
		return "", err
	}
	if focused == "" {
		return "", errors.New("no focused surface")
	}
	return focused, nil
}

type layoutSurfaceInfo struct {
	SurfaceID string `json:"surface_id"`
	AppID     string `json:"app_id"`
	Focused   bool   `json:"focused"`
}

type layoutInfo struct {
	Mode     string             `json:"mode"`
	Settings layoutInfoSettings `json:"settings"`
	Surfaces   []layoutSurfaceInfo `json:"surfaces"`
	Workspaces []struct {
		SurfaceOrder []string `json:"surface_order"`
	} `json:"workspaces"`
}

type layoutEnvelope struct {
	Layout layoutInfo `json:"layout"`
}

func fetchLayout() (layoutInfo, error) {
	raw, err := callCompositorControl(methodGetLayout, nil)
	if err != nil {
		return layoutInfo{}, err
	}
	var env layoutEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return layoutInfo{}, err
	}
	return env.Layout, nil
}

func focusedSurfaceID() (string, error) {
	info, err := fetchLayout()
	if err != nil {
		return "", err
	}
	for _, surface := range info.Surfaces {
		if surface.Focused {
			return surface.SurfaceID, nil
		}
	}
	return "", nil
}

func cycleString(list []string, current string) string {
	for index, value := range list {
		if value == current {
			return list[(index+1)%len(list)]
		}
	}
	return list[0]
}

// moveSurfaceRequest mirrors the bridge MoveSurfaceRequest.
type moveSurfaceRequest struct {
	SurfaceID     string `json:"surface_id"`
	Direction     string `json:"direction"`
	WaitTimeoutMs int    `json:"wait_timeout_ms,omitempty"`
}

func runSurfaceMove(args []string, stdout io.Writer, pretty bool) error {
	fs := flag.NewFlagSet("surface move", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	surfaceID := fs.String("surface", "", "surface id")
	direction := fs.String("direction", "", "direction: left, right, up, or down")
	timeoutMs := fs.Int("timeout-ms", 2000, "acknowledgement timeout in milliseconds")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *surfaceID == "" {
		return errors.New("--surface is required")
	}
	switch *direction {
	case "left", "right", "up", "down":
	default:
		return fmt.Errorf("invalid --direction %q (left|right|up|down)", *direction)
	}
	resolved, err := resolveSurfaceID(*surfaceID)
	if err != nil {
		return err
	}
	return callAndPrint(methodMoveSurface, moveSurfaceRequest{
		SurfaceID:     resolved,
		Direction:     *direction,
		WaitTimeoutMs: *timeoutMs,
	}, stdout, pretty)
}

// runSurfaceFocusCycle moves focus to the next/previous surface in the active
// workspace's surface order. Used by keyboard focus cycling.
func runSurfaceFocusCycle(action string, stdout io.Writer, pretty bool) error {
	info, err := fetchLayout()
	if err != nil {
		return err
	}
	var order []string
	if len(info.Workspaces) > 0 {
		order = info.Workspaces[0].SurfaceOrder
	}
	if len(order) == 0 {
		return errors.New("no surfaces to focus")
	}
	focused, _ := focusedSurfaceID()
	index := -1
	for i, id := range order {
		if id == focused {
			index = i
			break
		}
	}
	var target string
	if index < 0 {
		target = order[0]
	} else if action == "focus-next" {
		target = order[(index+1)%len(order)]
	} else {
		target = order[(index-1+len(order))%len(order)]
	}
	return callAndPrint(methodFocusSurface, surfaceRequest{SurfaceID: target}, stdout, pretty)
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
		ID          string `json:"id"`
		AppID       string `json:"app_id"`
		Title       string `json:"title"`
		Visible     bool   `json:"visible"`
		SurfaceKind string `json:"surface_kind"`
		OutputID    string `json:"output_id"`
	} `json:"surface"`
	Client struct {
		PID int `json:"pid"`
	} `json:"client"`
	Mapped          bool      `json:"mapped"`
	Visible         bool      `json:"visible"`
	InputInjectable bool      `json:"input_injectable"`
	UpdatedAt       time.Time `json:"updated_at"`
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
	OutputID      string `json:"output_id,omitempty"`
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
