package taskbarroute

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func newTestHandler(t *testing.T) (*Handler, string) {
	t.Helper()
	dir := t.TempDir()
	handler, err := New(Config{StateDir: dir})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return handler, filepath.Join(dir, fileName)
}

func do(t *testing.T, handler http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var r *http.Request
	if body == nil {
		r = httptest.NewRequest(method, path, nil)
	} else {
		payload, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		r = httptest.NewRequest(method, path, bytes.NewReader(payload))
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	return w
}

func TestPinsGetEmptyWhenNoFile(t *testing.T) {
	handler, _ := newTestHandler(t)
	w := do(t, handler, http.MethodGet, Path, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	var doc pinsDocument
	if err := json.Unmarshal(w.Body.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Apps) != 0 {
		t.Fatalf("expected empty, got %v", doc.Apps)
	}
}

func TestPinsPutPersistsAndDedupes(t *testing.T) {
	handler, file := newTestHandler(t)
	w := do(t, handler, http.MethodPut, Path, pinsDocument{Apps: []string{"  Alacritty.desktop ", "alacritty", "chromium", ""}})
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body %s", w.Code, w.Body.String())
	}
	var doc pinsDocument
	json.Unmarshal(w.Body.Bytes(), &doc)
	if len(doc.Apps) != 2 || doc.Apps[0] != "Alacritty.desktop" || doc.Apps[1] != "chromium" {
		t.Fatalf("got %v", doc.Apps)
	}

	// a fresh handler over the same file reads it back
	reload, _ := New(Config{StateDir: filepath.Dir(file)})
	w2 := do(t, reload, http.MethodGet, Path, nil)
	json.Unmarshal(w2.Body.Bytes(), &doc)
	if len(doc.Apps) != 2 || doc.Apps[1] != "chromium" {
		t.Fatalf("reload got %v", doc.Apps)
	}
}

func TestPinsRejectsOverflow(t *testing.T) {
	dir := t.TempDir()
	handler, err := New(Config{StateDir: dir, MaxPins: 2})
	if err != nil {
		t.Fatal(err)
	}
	w := do(t, handler, http.MethodPut, Path, pinsDocument{Apps: []string{"a", "b", "c"}})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d %s", w.Code, w.Body.String())
	}
}

func TestPinsEnforcesRoute(t *testing.T) {
	handler, _ := newTestHandler(t)
	w := do(t, handler, http.MethodGet, "/api/elsewhere", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
	w = do(t, handler, http.MethodDelete, Path, nil)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}
