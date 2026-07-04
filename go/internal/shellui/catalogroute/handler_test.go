package catalogroute

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"agora-de.local/go/internal/shellui/catalog"
)

func TestHandlerServesCatalogViews(t *testing.T) {
	handler := New(func(*http.Request) ([]catalog.AppView, error) {
		return []catalog.AppView{{ID: "browser", Name: "Browser", Icon: "browser"}}, nil
	})

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, AppsPath, nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if contentType := recorder.Header().Get("Content-Type"); contentType != "application/json" {
		t.Fatalf("content-type = %q, want application/json", contentType)
	}
	var raw struct {
		Apps []map[string]any `json:"apps"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	if len(raw.Apps) != 1 {
		t.Fatalf("json apps = %d, want 1", len(raw.Apps))
	}
	if raw.Apps[0]["id"] != "browser" {
		t.Fatalf("json app id = %q, want browser", raw.Apps[0]["id"])
	}
	if raw.Apps[0]["launchable"] != nil {
		t.Fatalf("unexpected launchable field from test provider: %+v", raw.Apps[0])
	}

	var response struct {
		Apps []catalog.AppView `json:"apps"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Apps) != 1 {
		t.Fatalf("apps = %d, want 1", len(response.Apps))
	}
	if response.Apps[0].ID != "browser" {
		t.Fatalf("app id = %q, want browser", response.Apps[0].ID)
	}
}

func TestHandlerLaunchesCatalogApp(t *testing.T) {
	handler := New(
		func(*http.Request) ([]catalog.AppView, error) {
			return []catalog.AppView{{ID: "browser", Name: "Browser", Icon: "browser", Launchable: true}}, nil
		},
		func(_ *http.Request, request LaunchRequest) (LaunchResult, error) {
			if request.AppID != "browser" {
				t.Fatalf("app id = %q, want browser", request.AppID)
			}
			return LaunchResult{AppID: request.AppID, LaunchID: "launch-1", SurfaceID: "view-1", Status: "launched"}, nil
		},
	)

	recorder := httptest.NewRecorder()
	body := strings.NewReader(`{"appId":"browser"}`)
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, LaunchPath, body))

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusAccepted)
	}
	var response LaunchResult
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.LaunchID != "launch-1" || response.SurfaceID != "view-1" {
		t.Fatalf("unexpected launch response: %+v", response)
	}
}

func TestLaunchRejectsBadRequests(t *testing.T) {
	handler := New(
		func(*http.Request) ([]catalog.AppView, error) { return nil, nil },
		func(*http.Request, LaunchRequest) (LaunchResult, error) {
			t.Fatal("launch provider should not be called")
			return LaunchResult{}, nil
		},
	)

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, LaunchPath, strings.NewReader(`{}`)))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}

func TestHandlerRejectsNonGet(t *testing.T) {
	handler := New(func(*http.Request) ([]catalog.AppView, error) {
		return nil, nil
	})

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, AppsPath, nil))

	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusMethodNotAllowed)
	}
	if allow := recorder.Header().Get("Allow"); allow != http.MethodGet {
		t.Fatalf("allow = %q, want GET", allow)
	}
}

func TestHandlerRejectsUnexpectedPath(t *testing.T) {
	handler := New(func(*http.Request) ([]catalog.AppView, error) {
		t.Fatal("provider should not be called for unexpected path")
		return nil, nil
	})

	for _, path := range []string{"/api/catalog/apps/", "/api/catalog/../apps", "/api/catalog/%2e%2e/apps"} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		if recorder.Code != http.StatusNotFound {
			t.Fatalf("path %q status = %d, want %d", path, recorder.Code, http.StatusNotFound)
		}
	}
}

func TestHandlerFailsClosedWhenProviderUnavailable(t *testing.T) {
	handler := New(func(*http.Request) ([]catalog.AppView, error) {
		return nil, errors.New("catalog source unavailable")
	})

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, AppsPath, nil))

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusServiceUnavailable)
	}
	var response struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Error != "catalog unavailable" {
		t.Fatalf("error = %q, want catalog unavailable", response.Error)
	}
}

func TestHandlerFailsClosedWithoutProvider(t *testing.T) {
	recorder := httptest.NewRecorder()
	Handler{}.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, AppsPath, nil))

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusServiceUnavailable)
	}
}
