// Package taskbarroute persists the user's pinned-taskbar app set and exposes it
// over HTTP so the panel (a TS shell expression) can read/write pins without
// owning persistence (AGENTS.md: server owns persistence, TS owns expression).
//
// GET  /api/taskbar/pins -> {"apps": ["alacritty", ...]}
// PUT  /api/taskbar/pins {"apps": [...]} -> {"apps": [...]}  (replaces the set)
package taskbarroute

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Path is the HTTP route for the pinned-apps resource.
const Path = "/api/taskbar/pins"

const fileName = "pinned-apps.json"

// Config configures the taskbar pin store.
type Config struct {
	// StateDir is the directory holding pinned-apps.json (default $HOME/.config/agora-de).
	StateDir string
	// MaxPins caps the pinned set (defense against a runaway payload).
	MaxPins int
}

// Handler serves the pinned-apps resource.
type Handler struct {
	mu       sync.Mutex
	filePath string
	maxPins  int
}

// New builds a Handler. StateDir defaults to $HOME/.config/agora-de when empty.
func New(config Config) (*Handler, error) {
	stateDir := strings.TrimSpace(config.StateDir)
	if stateDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("resolve state dir: %w", err)
		}
		stateDir = filepath.Join(home, ".config", "agora-de")
	}
	maxPins := config.MaxPins
	if maxPins <= 0 {
		maxPins = 64
	}
	return &Handler{
		filePath: filepath.Join(stateDir, fileName),
		maxPins:  maxPins,
	}, nil
}

type pinsDocument struct {
	Apps []string `json:"apps"`
}

func (h *Handler) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	if request.URL.Path != Path {
		http.NotFound(response, request)
		return
	}
	switch request.Method {
	case http.MethodGet:
		h.serveGet(response)
	case http.MethodPut, http.MethodPost:
		h.serveSet(response, request)
	default:
		response.Header().Set("Allow", "GET, PUT")
		writeError(response, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handler) serveGet(response http.ResponseWriter) {
	h.mu.Lock()
	defer h.mu.Unlock()
	doc, err := h.readLocked()
	if err != nil {
		writeError(response, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(response, http.StatusOK, doc)
}

func (h *Handler) serveSet(response http.ResponseWriter, request *http.Request) {
	var doc pinsDocument
	if err := json.NewDecoder(request.Body).Decode(&doc); err != nil {
		writeError(response, http.StatusBadRequest, "invalid pins request")
		return
	}
	cleaned, err := normalizeApps(doc.Apps, h.maxPins)
	if err != nil {
		writeError(response, http.StatusBadRequest, err.Error())
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if err := h.writeLocked(cleaned); err != nil {
		writeError(response, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(response, http.StatusOK, pinsDocument{Apps: cleaned})
}

// readLocked reads the pinned apps; a missing file is an empty set (not an error).
func (h *Handler) readLocked() (pinsDocument, error) {
	data, err := os.ReadFile(h.filePath)
	if errors.Is(err, os.ErrNotExist) {
		return pinsDocument{Apps: []string{}}, nil
	}
	if err != nil {
		return pinsDocument{}, err
	}
	var doc pinsDocument
	if err := json.Unmarshal(data, &doc); err != nil {
		return pinsDocument{}, fmt.Errorf("parse %s: %w", h.filePath, err)
	}
	cleaned, err := normalizeApps(doc.Apps, h.maxPins)
	if err != nil {
		return pinsDocument{}, err
	}
	return pinsDocument{Apps: cleaned}, nil
}

func (h *Handler) writeLocked(apps []string) error {
	if err := os.MkdirAll(filepath.Dir(h.filePath), 0o755); err != nil {
		return err
	}
	doc := pinsDocument{Apps: apps}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(h.filePath, data, 0o644)
}

// normalizeApps trims, dedupes (case-insensitive, preserving first-seen order),
// drops empties, and enforces the cap.
func normalizeApps(apps []string, maxPins int) ([]string, error) {
	seen := make(map[string]bool, len(apps))
	cleaned := make([]string, 0, len(apps))
	for _, app := range apps {
		key := normalizeAppKey(app)
		if key == "" || seen[key] {
			continue
		}
		if len(cleaned) >= maxPins {
			return nil, fmt.Errorf("too many pinned apps (max %d)", maxPins)
		}
		seen[key] = true
		cleaned = append(cleaned, strings.TrimSpace(app))
	}
	return cleaned, nil
}

func normalizeAppKey(app string) string {
	return strings.ToLower(strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(app), ".desktop")))
}

func writeJSON(response http.ResponseWriter, status int, body any) {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(body)
}

func writeError(response http.ResponseWriter, status int, message string) {
	writeJSON(response, status, map[string]string{"error": message})
}
