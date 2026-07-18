package main

// Layout session save/restore (#5731): snapshot the current app+WM arrangement
// to a named file, and restore it by re-launching the apps and reproducing the
// saved layout settings + window order + focus.
//
// Snapshots live under $AGORA_DE_STATE_DIR/layouts/<name>.json (default
// ~/.config/agora-de/layouts). The snapshot stores: layout mode + settings, the
// ordered list of app ids (the workspace surface_order mapped to app_id), and
// the focused app id. Geometry is NOT stored — it is derived by the planner from
// order + settings + output, so restoring order + settings reproduces it.

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const layoutSessionsSubdir = "layouts"

// layoutInfoSettings mirrors the settings block returned by `layout get`.
type layoutInfoSettings struct {
	Rule        string            `json:"rule"`
	Mode        string            `json:"mode"`
	MasterCount int               `json:"master_count"`
	MasterRatio float64           `json:"master_ratio"`
	SmartGaps   bool              `json:"smart_gaps"`
	Gaps        layoutSessionGaps `json:"gaps"`
}

func (s layoutInfoSettings) toSession() layoutSessionSettings {
	return layoutSessionSettings{
		Rule:        s.Rule,
		Mode:        s.Mode,
		MasterCount: s.MasterCount,
		MasterRatio: s.MasterRatio,
		SmartGaps:   s.SmartGaps,
		Gaps:        s.Gaps,
	}
}

// layoutSession is the on-disk snapshot.
type layoutSession struct {
	Name       string             `json:"name"`
	SavedAt    string             `json:"saved_at"`
	Mode       string             `json:"mode"`
	Settings   layoutSessionSettings `json:"settings"`
	Order      []string           `json:"order"`        // app_ids in surface order (master-first)
	FocusedApp string             `json:"focused_app"`  // app_id of the focused surface
}

type layoutSessionSettings struct {
	Rule        string                `json:"rule"`
	Mode        string                `json:"mode"`
	MasterCount int                   `json:"master_count"`
	MasterRatio float64               `json:"master_ratio"`
	SmartGaps   bool                  `json:"smart_gaps"`
	Gaps        layoutSessionGaps     `json:"gaps"`
}

type layoutSessionGaps struct {
	OuterHorizontal int `json:"outer_horizontal"`
	OuterVertical   int `json:"outer_vertical"`
	InnerHorizontal int `json:"inner_horizontal"`
	InnerVertical   int `json:"inner_vertical"`
}

func stateDir() string {
	if v := strings.TrimSpace(os.Getenv("AGORA_DE_STATE_DIR")); v != "" {
		return v
	}
	if home := strings.TrimSpace(os.Getenv("HOME")); home != "" {
		return filepath.Join(home, ".config", "agora-de")
	}
	return filepath.Join("/home/agent", ".config", "agora-de")
}

func layoutSessionsDir() string {
	return filepath.Join(stateDir(), layoutSessionsSubdir)
}

// requireSessionName parses --name for a layout session subcommand.
func requireSessionName(action string, args []string) (string, error) {
	fs := flag.NewFlagSet("layout "+action, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	name := fs.String("name", "", "layout session name")
	if err := fs.Parse(args); err != nil {
		return "", err
	}
	if strings.TrimSpace(*name) == "" {
		return "", errors.New("--name is required")
	}
	return *name, nil
}

func layoutSessionPath(name string) (string, error) {
	clean := strings.TrimSpace(name)
	if clean == "" {
		return "", errors.New("layout name is required")
	}
	// keep the filename bounded to the name (no path traversal)
	base := strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == ':' {
			return '_'
		}
		return r
	}, clean) + ".json"
	return filepath.Join(layoutSessionsDir(), base), nil
}

// captureLayoutSession builds a snapshot from the current layout state.
func captureLayoutSession(name string) (layoutSession, error) {
	info, err := fetchLayout()
	if err != nil {
		return layoutSession{}, err
	}
	surfacesByApp := map[string]bool{}
	for _, s := range info.Surfaces {
		if s.AppID != "" {
			surfacesByApp[s.AppID] = true
		}
	}
	// surface_order is a list of surface ids; map each to its app_id.
	idToApp := map[string]string{}
	for _, s := range info.Surfaces {
		idToApp[s.SurfaceID] = s.AppID
	}
	var order []string
	if len(info.Workspaces) > 0 {
		for _, sid := range info.Workspaces[0].SurfaceOrder {
			if app := idToApp[sid]; app != "" {
				order = append(order, app)
			}
		}
	}
	focusedApp := ""
	for _, s := range info.Surfaces {
		if s.Focused && s.AppID != "" {
			focusedApp = s.AppID
			break
		}
	}
	if len(order) == 0 {
		return layoutSession{}, errors.New("no tiled surfaces to snapshot")
	}
	return layoutSession{
		Name:       name,
		SavedAt:    time.Now().UTC().Format(time.RFC3339),
		Mode:       firstNonEmptyStr(info.Mode, info.Settings.Mode),
		Settings:   info.Settings.toSession(),
		Order:      order,
		FocusedApp: focusedApp,
	}, nil
}

func saveLayoutSession(name string) error {
	session, err := captureLayoutSession(name)
	if err != nil {
		return err
	}
	path, err := layoutSessionPath(name)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return err
	}
	fmt.Printf("saved layout %q (%d app%s) -> %s\n", name, len(session.Order), plural(len(session.Order)), path)
	return nil
}

func loadLayoutSession(name string) (layoutSession, error) {
	path, err := layoutSessionPath(name)
	if err != nil {
		return layoutSession{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return layoutSession{}, fmt.Errorf("load layout %q: %w", name, err)
	}
	var session layoutSession
	if err := json.Unmarshal(data, &session); err != nil {
		return layoutSession{}, fmt.Errorf("parse %s: %w", path, err)
	}
	return session, nil
}

func listLayoutSessions() error {
	dir := layoutSessionsDir()
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		fmt.Println("(no saved layouts)")
		return nil
	}
	if err != nil {
		return err
	}
	names := []string{}
	for _, e := range entries {
		if !e.Type().IsRegular() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		names = append(names, strings.TrimSuffix(e.Name(), ".json"))
	}
	sort.Strings(names)
	if len(names) == 0 {
		fmt.Println("(no saved layouts)")
		return nil
	}
	for _, n := range names {
		s, err := loadLayoutSession(n)
		if err != nil || len(s.Order) == 0 {
			fmt.Printf("%s\n", n)
			continue
		}
		fmt.Printf("%s\t[%s/%s]\tapps=%d%s\n", n, firstNonEmptyStr(s.Settings.Rule, "?"), firstNonEmptyStr(s.Mode, "?"), len(s.Order), focusedTag(s.FocusedApp))
	}
	return nil
}

func deleteLayoutSession(name string) error {
	path, err := layoutSessionPath(name)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("delete layout %q: %w", name, err)
	}
	fmt.Printf("deleted layout %q\n", name)
	return nil
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func focusedTag(app string) string {
	if app == "" {
		return ""
	}
	return " focus=" + app
}

func firstNonEmptyStr(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
