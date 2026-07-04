package catalogroute

import (
	"encoding/json"
	"net/http"

	"agora-de.local/go/internal/shellui/catalog"
)

const AppsPath = "/api/catalog/apps"
const LaunchPath = "/api/catalog/launch"

type Provider func(*http.Request) ([]catalog.AppView, error)
type LaunchProvider func(*http.Request, LaunchRequest) (LaunchResult, error)

type Handler struct {
	Provider       Provider
	LaunchProvider LaunchProvider
}

type appsResponse struct {
	Apps []catalog.AppView `json:"apps"`
}

type LaunchRequest struct {
	AppID string `json:"appId"`
}

type LaunchResult struct {
	AppID     string `json:"appId"`
	LaunchID  string `json:"launchId,omitempty"`
	SurfaceID string `json:"surfaceId,omitempty"`
	Status    string `json:"status"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func New(provider Provider, launchProvider ...LaunchProvider) Handler {
	handler := Handler{Provider: provider}
	if len(launchProvider) > 0 {
		handler.LaunchProvider = launchProvider[0]
	}
	return handler
}

func (handler Handler) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	switch request.URL.Path {
	case AppsPath:
		handler.serveApps(response, request)
	case LaunchPath:
		handler.serveLaunch(response, request)
	default:
		http.NotFound(response, request)
	}
}

func (handler Handler) serveApps(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		response.Header().Set("Allow", http.MethodGet)
		writeJSON(response, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	if handler.Provider == nil {
		writeJSON(response, http.StatusServiceUnavailable, errorResponse{Error: "catalog unavailable"})
		return
	}

	views, err := handler.Provider(request)
	if err != nil {
		writeJSON(response, http.StatusServiceUnavailable, errorResponse{Error: "catalog unavailable"})
		return
	}

	writeJSON(response, http.StatusOK, appsResponse{Apps: views})
}

func (handler Handler) serveLaunch(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		response.Header().Set("Allow", http.MethodPost)
		writeJSON(response, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	if handler.LaunchProvider == nil {
		writeJSON(response, http.StatusServiceUnavailable, errorResponse{Error: "launch unavailable"})
		return
	}

	var launch LaunchRequest
	if err := json.NewDecoder(request.Body).Decode(&launch); err != nil {
		writeJSON(response, http.StatusBadRequest, errorResponse{Error: "invalid launch request"})
		return
	}
	if launch.AppID == "" {
		writeJSON(response, http.StatusBadRequest, errorResponse{Error: "appId is required"})
		return
	}

	result, err := handler.LaunchProvider(request, launch)
	if err != nil {
		writeJSON(response, http.StatusServiceUnavailable, errorResponse{Error: "launch failed"})
		return
	}
	writeJSON(response, http.StatusAccepted, result)
}

func writeJSON(response http.ResponseWriter, status int, value any) {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(value)
}
