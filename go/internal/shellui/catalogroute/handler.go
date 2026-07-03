package catalogroute

import (
	"encoding/json"
	"net/http"

	"agora-de.local/go/internal/shellui/catalog"
)

const AppsPath = "/api/catalog/apps"

type Provider func(*http.Request) ([]catalog.AppView, error)

type Handler struct {
	Provider Provider
}

type appsResponse struct {
	Apps []catalog.AppView `json:"apps"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func New(provider Provider) Handler {
	return Handler{Provider: provider}
}

func (handler Handler) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	if request.URL.Path != AppsPath {
		http.NotFound(response, request)
		return
	}
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

func writeJSON(response http.ResponseWriter, status int, value any) {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(value)
}
