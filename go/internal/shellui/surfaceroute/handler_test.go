package surfaceroute

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"agora-de.local/go/internal/shellui/surfaces"
)

func TestHandlerServesSurfaceViews(t *testing.T) {
	handler := New(func(*http.Request) ([]surfaces.SurfaceView, error) {
		return []surfaces.SurfaceView{{
			ID:               "view-42",
			OwnerUID:         60001,
			Mapped:           true,
			Focused:          true,
			InputDeniedCount: 1,
		}}, nil
	})

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, SurfacesPath, nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if contentType := recorder.Header().Get("Content-Type"); contentType != "application/json" {
		t.Fatalf("content-type = %q, want application/json", contentType)
	}

	var response struct {
		Surfaces []surfaces.SurfaceView `json:"surfaces"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Surfaces) != 1 {
		t.Fatalf("surfaces = %d, want 1", len(response.Surfaces))
	}
	surface := response.Surfaces[0]
	if surface.ID != "view-42" || surface.OwnerUID != 60001 || !surface.Mapped || !surface.Focused || surface.InputDeniedCount != 1 {
		t.Fatalf("unexpected surface response: %+v", surface)
	}
}

func TestHandlerServesCustomPath(t *testing.T) {
	handler := Handler{
		Path: "/api/work-surface-controls",
		Provider: func(*http.Request) ([]surfaces.SurfaceView, error) {
			return []surfaces.SurfaceView{{ID: "view-42", OwnerUID: 60001}}, nil
		},
	}

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/work-surface-controls", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
}

func TestHandlerRejectsNonGet(t *testing.T) {
	handler := New(func(*http.Request) ([]surfaces.SurfaceView, error) {
		return nil, nil
	})

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, SurfacesPath, nil))

	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusMethodNotAllowed)
	}
	if allow := recorder.Header().Get("Allow"); allow != http.MethodGet {
		t.Fatalf("allow = %q, want GET", allow)
	}
}

func TestHandlerRejectsUnexpectedPath(t *testing.T) {
	handler := New(func(*http.Request) ([]surfaces.SurfaceView, error) {
		t.Fatal("provider should not be called for unexpected path")
		return nil, nil
	})

	for _, path := range []string{"/api/surfaces/", "/api/../surfaces", "/api/%2e%2e/surfaces"} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		if recorder.Code != http.StatusNotFound {
			t.Fatalf("path %q status = %d, want %d", path, recorder.Code, http.StatusNotFound)
		}
	}
}

func TestHandlerFailsClosedWhenProviderUnavailable(t *testing.T) {
	handler := New(func(*http.Request) ([]surfaces.SurfaceView, error) {
		return nil, errors.New("surface source unavailable")
	})

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, SurfacesPath, nil))

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusServiceUnavailable)
	}
	var response struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Error != "surfaces unavailable" {
		t.Fatalf("error = %q, want surfaces unavailable", response.Error)
	}
}

func TestHandlerFailsClosedWithoutProvider(t *testing.T) {
	recorder := httptest.NewRecorder()
	Handler{}.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, SurfacesPath, nil))

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusServiceUnavailable)
	}
}
