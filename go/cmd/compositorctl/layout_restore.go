package main

// Layout session restore (#5731): re-launch the saved apps and reproduce the
// saved layout settings + window order + focus. Apps are launched via their
// desktop entry Exec (resolved by app id), falling back to the bare app id as a
// command. Window arrangement is reproduced by setting the bridge surface order
// (set_surface_order) to the saved order; the planner recomputes geometry.

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

var desktopEntryRoots = []string{
	"/usr/share/applications",
	"/usr/local/share/applications",
	filepath.Join(os.Getenv("HOME"), ".local/share/applications"),
}

// resolveLaunchArgv maps an app id to a launch argv: prefers a matching desktop
// entry's Exec (with field codes stripped), else treats the id as a bare command.
func resolveLaunchArgv(appID string) []string {
	if argv := argvFromDesktopEntry(appID); len(argv) > 0 {
		return argv
	}
	return []string{appID}
}

func argvFromDesktopEntry(appID string) []string {
	lower := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(appID), ".desktop"))
	for _, root := range desktopEntryRoots {
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			name := entry.Name()
			if !strings.HasSuffix(name, ".desktop") {
				continue
			}
			stem := strings.ToLower(strings.TrimSuffix(name, ".desktop"))
			match := stem == lower
			execLine, wmClass, nameField := "", "", ""
			if f, err := os.Open(filepath.Join(root, name)); err == nil {
				scanner := bufio.NewScanner(f)
				for scanner.Scan() {
					line := strings.TrimSpace(scanner.Text())
					if strings.HasPrefix(line, "Exec=") {
						execLine = strings.TrimSpace(strings.TrimPrefix(line, "Exec="))
					}
					if strings.HasPrefix(line, "StartupWMClass=") {
						wmClass = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(line, "StartupWMClass=")))
					}
					if strings.HasPrefix(line, "Name=") {
						nameField = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(line, "Name=")))
					}
				}
				f.Close()
				if execLine == "" || strings.Contains(execLine, "NoDisplay=true") {
					continue
				}
				if !match {
					match = wmClass == lower || nameField == lower
				}
				if match {
					return splitDesktopExec(execLine)
				}
			}
		}
	}
	return nil
}

// splitDesktopExec parses a .desktop Exec value into argv, stripping freedesktop
// field codes (%f %F %u %U %d %D %n %N %i %c %k). Simple shell-ish split
// respecting single/double quotes — sufficient for typical Exec lines.
func splitDesktopExec(execLine string) []string {
	// drop everything from the first field code onward (they are trailing args)
	if i := strings.IndexAny(execLine, "%"); i >= 0 {
		execLine = strings.TrimSpace(execLine[:i])
	}
	var argv []string
	var current strings.Builder
	inSingle, inDouble := false, false
	flush := func() {
		if current.Len() > 0 {
			argv = append(argv, current.String())
			current.Reset()
		}
	}
	for i := 0; i < len(execLine); i++ {
		c := execLine[i]
		switch {
		case c == '\\' && i+1 < len(execLine):
			current.WriteByte(execLine[i+1])
			i++
		case c == '"' && !inSingle:
			inDouble = !inDouble
		case c == '\'' && !inDouble:
			inSingle = !inSingle
		case c == ' ' || c == '\t':
			if inSingle || inDouble {
				current.WriteByte(c)
			} else {
				flush()
			}
		default:
			current.WriteByte(c)
		}
	}
	flush()
	return argv
}

// launchAppForRestore spawns argv and waits for a matching surface, returning the
// mapped surface id. Mirrors runLaunch's spawn+wait core.
func launchAppForRestore(argv []string, expectedAppID string) (string, error) {
	if len(argv) == 0 {
		return "", errors.New("empty argv")
	}
	startedAt := time.Now()
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("launch %s: %w", argv[0], err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	observation, err := waitForSurface(launchSurfaceMatch{
		RootPID:       cmd.Process.Pid,
		StartedAt:     startedAt,
		ExpectedAppID: expectedAppID,
		ReusableIDs:   reusableSurfaceIDs(launchSurfaceMatch{StartedAt: startedAt, ExpectedAppID: expectedAppID}),
	}, 8*time.Second, done)
	if err != nil {
		return "", err
	}
	if observation.Surface.Surface.ID == "" {
		return "", fmt.Errorf("launched %s but no surface mapped (status=%s)", argv[0], observation.Status)
	}
	return observation.Surface.Surface.ID, nil
}

// restoreLayoutSession re-launches the saved apps and reproduces the arrangement.
func restoreLayoutSession(name string) error {
	session, err := loadLayoutSession(name)
	if err != nil {
		return err
	}
	fmt.Printf("restoring layout %q (%d app%s, %s/%s)...\n", name, len(session.Order), plural(len(session.Order)), firstNonEmptyStr(session.Settings.Rule, "?"), firstNonEmptyStr(session.Mode, "?"))

	// 1. re-apply layout mode + settings so the planner reproduces the columns.
	if mode := firstNonEmptyStr(session.Mode, session.Settings.Mode); mode != "" {
		if _, err := callCompositorControl(methodSetLayoutMode, setLayoutModeRequest{Mode: mode}); err != nil {
			fmt.Fprintf(os.Stderr, "warn: set-mode %q failed: %v\n", mode, err)
		}
	}
	if err := applySessionSettings(session); err != nil {
		fmt.Fprintf(os.Stderr, "warn: set-settings failed: %v\n", err)
	}

	// 2. launch one instance per saved order entry (order-preserving; repeats are
	// allowed so multiple windows of the same app are reproduced by launch order).
	type placed struct{ appID, surfaceID string }
	var placedApps []placed
	var order []string
	for _, appID := range session.Order {
		argv := resolveLaunchArgv(appID)
		surfaceID, launchErr := launchAppForRestore(argv, appID)
		if launchErr != nil {
			fmt.Fprintf(os.Stderr, "warn: could not restore app %q (%v) — skipped\n", appID, launchErr)
			continue
		}
		placedApps = append(placedApps, placed{appID: appID, surfaceID: surfaceID})
		order = append(order, surfaceID)
		time.Sleep(400 * time.Millisecond) // let the previous app settle before the next
	}

	// 3. reproduce the saved window order.
	if len(order) > 0 {
		req := map[string]any{"surface_ids": order}
		if _, err := callCompositorControl(methodSetSurfaceOrder, req); err != nil {
			fmt.Fprintf(os.Stderr, "warn: set-surface-order failed: %v\n", err)
		}
	}

	// 4. restore focus (first placed window matching the saved focused app).
	for _, p := range placedApps {
		if p.appID == session.FocusedApp {
			if _, err := callCompositorControl(methodFocusSurface, surfaceRequest{SurfaceID: p.surfaceID}); err != nil {
				fmt.Fprintf(os.Stderr, "warn: focus %s failed: %v\n", p.surfaceID, err)
			}
			break
		}
	}
	fmt.Printf("restored layout %q (%d/%d apps placed)\n", name, len(order), len(session.Order))
	return nil
}

func applySessionSettings(session layoutSession) error {
	s := session.Settings
	req := updateLayoutSettingsRequest{}
	changed := false
	if v := strings.TrimSpace(s.Rule); v != "" {
		req.Rule = &v
		changed = true
	}
	if v := strings.TrimSpace(s.Mode); v != "" {
		req.Mode = &v
		changed = true
	}
	if s.MasterCount > 0 {
		v := s.MasterCount
		req.MasterCount = &v
		changed = true
	}
	if s.MasterRatio > 0 {
		v := s.MasterRatio
		req.MasterRatio = &v
		changed = true
	}
	req.SmartGaps = &s.SmartGaps
	changed = true
	req.Gaps = &layoutGaps{
		OuterHorizontal: s.Gaps.OuterHorizontal,
		OuterVertical:   s.Gaps.OuterVertical,
		InnerHorizontal: s.Gaps.InnerHorizontal,
		InnerVertical:   s.Gaps.InnerVertical,
	}
	if !changed {
		return nil
	}
	_, err := callCompositorControl(methodUpdateLayoutSettings, req)
	return err
}
