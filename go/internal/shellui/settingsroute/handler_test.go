package settingsroute

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"agora-de.local/go/internal/settingsprotocol"
	"agora-de.local/go/internal/settingsregistry"
)

type fixtureModule struct {
	id      string
	delay   time.Duration
	panics  bool
	request chan string
}

func (module fixtureModule) Manifest() settingsprotocol.SettingsModuleManifest {
	return settingsprotocol.SettingsModuleManifest{
		ID:              module.id,
		Category:        settingsprotocol.SettingsCategorySystem,
		Title:           module.id,
		Summary:         "route fixture",
		Icon:            "settings",
		Route:           module.id,
		SearchTerms:     []string{},
		Capabilities:    []settingsprotocol.SettingsCapability{settingsprotocol.SettingsCapabilityLoad, settingsprotocol.SettingsCapabilityApply},
		BackendAdapter:  module.id,
		UIEntryPoint:    module.id,
		ContractVersion: 1,
	}
}

func (fixtureModule) Availability(context.Context) settingsprotocol.SettingsModuleAvailability {
	return settingsprotocol.SettingsModuleAvailability{State: settingsprotocol.SettingsAvailabilityAvailable}
}

func (module fixtureModule) Handler() http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if module.panics {
			panic("fixture failure")
		}
		if module.delay > 0 {
			time.Sleep(module.delay)
		}
		if module.request != nil {
			module.request <- request.Method + " " + request.URL.Path
		}
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(map[string]string{"module": module.id})
	})
}

func fixtureHandler(t *testing.T, timeout time.Duration, modules ...settingsregistry.Module) http.Handler {
	t.Helper()
	registry, err := settingsregistry.New(modules, timeout)
	if err != nil {
		t.Fatal(err)
	}
	return New(registry)
}

func serve(handler http.Handler, method string, path string, body string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	if method == http.MethodPost {
		request.Header.Set("Content-Type", "application/json")
	}
	handler.ServeHTTP(recorder, request)
	return recorder
}

func TestCatalogAndModuleDispatchAreIndependent(t *testing.T) {
	requests := make(chan string, 1)
	handler := fixtureHandler(t, time.Second,
		fixtureModule{id: "healthy", request: requests},
		fixtureModule{id: "second"},
	)

	catalogResponse := serve(handler, http.MethodGet, CatalogPath, "")
	if catalogResponse.Code != http.StatusOK {
		t.Fatalf("catalog status = %d: %s", catalogResponse.Code, catalogResponse.Body.String())
	}
	var catalog settingsprotocol.SettingsCatalogResponse
	if err := json.Unmarshal(catalogResponse.Body.Bytes(), &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Modules) != 2 || catalog.Modules[0].Manifest.ID != "healthy" {
		t.Fatalf("unexpected catalog: %+v", catalog)
	}

	loadResponse := serve(handler, http.MethodGet, ModulesPrefix+"healthy/load", "")
	if loadResponse.Code != http.StatusOK {
		t.Fatalf("load status = %d: %s", loadResponse.Code, loadResponse.Body.String())
	}
	if got := <-requests; got != "GET /state" {
		t.Fatalf("delegated request = %q, want GET /state", got)
	}
}

func TestAdapterTimeoutAndPanicBecomeTypedErrors(t *testing.T) {
	handler := fixtureHandler(t, 20*time.Millisecond,
		fixtureModule{id: "slow", delay: 250 * time.Millisecond},
		fixtureModule{id: "panic", panics: true},
		fixtureModule{id: "healthy"},
	)

	started := time.Now()
	timedOut := serve(handler, http.MethodGet, ModulesPrefix+"slow/load", "")
	if timedOut.Code != http.StatusGatewayTimeout || time.Since(started) > 200*time.Millisecond {
		t.Fatalf("timeout response = %d after %s: %s", timedOut.Code, time.Since(started), timedOut.Body.String())
	}
	var timeoutError settingsprotocol.SettingsError
	if err := json.Unmarshal(timedOut.Body.Bytes(), &timeoutError); err != nil {
		t.Fatal(err)
	}
	if timeoutError.Code != settingsprotocol.SettingsErrorTimeout {
		t.Fatalf("timeout error = %+v", timeoutError)
	}

	panicResponse := serve(handler, http.MethodGet, ModulesPrefix+"panic/load", "")
	if panicResponse.Code != http.StatusServiceUnavailable {
		t.Fatalf("panic response = %d: %s", panicResponse.Code, panicResponse.Body.String())
	}
	healthy := serve(handler, http.MethodGet, ModulesPrefix+"healthy/load", "")
	if healthy.Code != http.StatusOK {
		t.Fatalf("healthy module after failures = %d: %s", healthy.Code, healthy.Body.String())
	}
}

func TestGatewayRejectsUnknownOriginsModulesOperationsAndMediaTypes(t *testing.T) {
	handler := fixtureHandler(t, time.Second, fixtureModule{id: "healthy"})

	crossOriginRequest := httptest.NewRequest(http.MethodGet, CatalogPath, nil)
	crossOriginRequest.Header.Set("Origin", "https://evil.example")
	crossOrigin := httptest.NewRecorder()
	handler.ServeHTTP(crossOrigin, crossOriginRequest)
	if crossOrigin.Code != http.StatusForbidden {
		t.Fatalf("cross-origin status = %d", crossOrigin.Code)
	}

	unknown := serve(handler, http.MethodGet, ModulesPrefix+"unknown/load", "")
	if unknown.Code != http.StatusNotFound {
		t.Fatalf("unknown module status = %d", unknown.Code)
	}

	operation := serve(handler, http.MethodGet, ModulesPrefix+"healthy/command", "")
	if operation.Code != http.StatusNotFound {
		t.Fatalf("unknown operation status = %d", operation.Code)
	}

	wrongMethod := serve(handler, http.MethodGet, ModulesPrefix+"healthy/apply", "")
	if wrongMethod.Code != http.StatusMethodNotAllowed {
		t.Fatalf("wrong method status = %d", wrongMethod.Code)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, ModulesPrefix+"healthy/apply", strings.NewReader(`{}`))
	request.Header.Set("Content-Type", "text/plain")
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("wrong media type status = %d", recorder.Code)
	}
}
