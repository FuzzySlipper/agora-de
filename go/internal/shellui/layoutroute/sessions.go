package layoutroute

// Layout session HTTP route (#5731): lists saved layouts and dispatches
// save/restore/delete to compositorctl. Restore re-launches apps so it gets a
// generous timeout; the orchestration lives in compositorctl (layout_session.go
// / layout_restore.go).

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type sessionSummary struct {
	Name      string `json:"name"`
	Rule      string `json:"rule"`
	Mode      string `json:"mode"`
	AppCount  int    `json:"app_count"`
	FocusedApp string `json:"focused_app,omitempty"`
}

type sessionRequest struct {
	Action string `json:"action"` // save | restore | delete
	Name   string `json:"name"`
}

type sessionResponse struct {
	Action string          `json:"action"`
	Name   string          `json:"name"`
	Status string          `json:"status"`
	Output string          `json:"output,omitempty"`
	Items  []sessionSummary `json:"items,omitempty"`
}

// NewSessions serves the saved-layout collection.
func NewSessions(config Config) http.Handler {
	ctl := compositorctlPath(config.CompositorctlPath)
	stateDir := strings.TrimSpace(config.StateDir)
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != SessionsPath {
			http.NotFound(response, request)
			return
		}
		switch request.Method {
		case http.MethodGet:
			serveSessionList(response, stateDir)
		case http.MethodPost:
			serveSessionAction(response, request, ctl)
		default:
			response.Header().Set("Allow", "GET, POST")
			writeJSON(response, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		}
	})
}

func serveSessionList(response http.ResponseWriter, stateDir string) {
	dir := filepath.Join(stateDir, "layouts")
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) == 0 {
		writeJSON(response, http.StatusOK, sessionResponse{Items: []sessionSummary{}})
		return
	}
	var items []sessionSummary
	for _, entry := range entries {
		if !entry.Type().IsRegular() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ".json")
		summary := readSessionSummary(filepath.Join(dir, entry.Name()), name)
		items = append(items, summary)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	writeJSON(response, http.StatusOK, sessionResponse{Items: items})
}

func readSessionSummary(path, name string) sessionSummary {
	data, err := os.ReadFile(path)
	if err != nil {
		return sessionSummary{Name: name}
	}
	var doc struct {
		Mode     string `json:"mode"`
		Rule     string `json:"rule"`
		Order    []string `json:"order"`
		Focused  string `json:"focused_app"`
		Settings struct {
			Rule string `json:"rule"`
		} `json:"settings"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return sessionSummary{Name: name}
	}
	return sessionSummary{
		Name:       name,
		Rule:       firstNonEmpty(doc.Settings.Rule, doc.Rule),
		Mode:       doc.Mode,
		AppCount:   len(doc.Order),
		FocusedApp: doc.Focused,
	}
}

func serveSessionAction(response http.ResponseWriter, request *http.Request, ctl string) {
	var req sessionRequest
	if err := json.NewDecoder(request.Body).Decode(&req); err != nil {
		writeJSON(response, http.StatusBadRequest, errorResponse{Error: "invalid layout session request"})
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		writeJSON(response, http.StatusBadRequest, errorResponse{Error: "name is required"})
		return
	}
	subcommand := ""
	timeout := 5 * time.Second
	switch req.Action {
	case "save":
		subcommand = "save"
	case "restore":
		subcommand = "restore"
		timeout = 90 * time.Second // restore re-launches apps
	case "delete":
		subcommand = "delete"
	default:
		writeJSON(response, http.StatusBadRequest, errorResponse{Error: "action must be save|restore|delete"})
		return
	}
	args := []string{"layout", subcommand, "--name", name}
	ctx, cancel := context.WithTimeout(request.Context(), timeout)
	defer cancel()
	output, err := exec.CommandContext(ctx, ctl, args...).CombinedOutput()
	if err != nil {
		writeCompositorctlError(response, output, err)
		return
	}
	writeJSON(response, http.StatusAccepted, sessionResponse{
		Action: req.Action,
		Name:   name,
		Status: "accepted",
		Output: strings.TrimSpace(string(output)),
	})
}
