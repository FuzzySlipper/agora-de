package settingsroute

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"agora-de.local/go/internal/settingsprotocol"
	"agora-de.local/go/internal/settingsregistry"
)

const (
	Prefix        = "/api/settings/"
	CatalogPath   = "/api/settings/catalog"
	ModulesPrefix = "/api/settings/modules/"
)

type Handler struct {
	registry *settingsregistry.Registry
}

func New(registry *settingsregistry.Registry) http.Handler {
	return Handler{registry: registry}
}

func (handler Handler) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	if !sameOrigin(request) {
		writeError(response, http.StatusForbidden, settingsprotocol.SettingsError{
			Code:      settingsprotocol.SettingsErrorInvalidRequest,
			Message:   "cross-origin settings requests are forbidden",
			Retryable: false,
		})
		return
	}
	if request.URL.Path == CatalogPath {
		if request.Method != http.MethodGet {
			response.Header().Set("Allow", http.MethodGet)
			writeMethodError(response)
			return
		}
		writeJSON(response, http.StatusOK, handler.registry.Catalog(request.Context()))
		return
	}
	if !strings.HasPrefix(request.URL.Path, ModulesPrefix) {
		http.NotFound(response, request)
		return
	}

	remainder := strings.TrimPrefix(request.URL.Path, ModulesPrefix)
	parts := strings.Split(remainder, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		http.NotFound(response, request)
		return
	}
	module, ok := handler.registry.Module(parts[0])
	if !ok {
		writeError(response, http.StatusNotFound, settingsprotocol.SettingsError{
			Code:      settingsprotocol.SettingsErrorUnsupported,
			Message:   "settings module is not registered",
			Retryable: false,
		})
		return
	}
	operation := parts[1]
	method, ok := operationMethod(operation)
	if !ok {
		http.NotFound(response, request)
		return
	}
	if request.Method != method {
		response.Header().Set("Allow", method)
		writeMethodError(response)
		return
	}
	if method == http.MethodPost && !jsonContentType(request.Header.Get("Content-Type")) {
		writeError(response, http.StatusUnsupportedMediaType, settingsprotocol.SettingsError{
			Code:      settingsprotocol.SettingsErrorInvalidRequest,
			Message:   "settings mutations require application/json",
			Retryable: false,
		})
		return
	}

	moduleRequest := request.Clone(request.Context())
	moduleURL := *request.URL
	moduleURL.Path = "/" + moduleOperationPath(operation)
	moduleRequest.URL = &moduleURL
	handler.delegate(response, moduleRequest, module.Handler(), handler.registry.Timeout())
}

func (handler Handler) delegate(response http.ResponseWriter, request *http.Request, module http.Handler, timeout time.Duration) {
	ctx, cancel := contextWithTimeout(request, timeout)
	defer cancel()
	request = request.Clone(ctx)
	buffered := newBufferedResponse()
	done := make(chan any, 1)
	go func() {
		defer func() { done <- recover() }()
		module.ServeHTTP(buffered, request)
	}()

	select {
	case recovered := <-done:
		if recovered != nil {
			writeError(response, http.StatusServiceUnavailable, settingsprotocol.SettingsError{
				Code:      settingsprotocol.SettingsErrorApplyFailed,
				Message:   "settings adapter failed",
				Retryable: true,
			})
			return
		}
		buffered.copyTo(response)
	case <-ctx.Done():
		writeError(response, http.StatusGatewayTimeout, settingsprotocol.SettingsError{
			Code:      settingsprotocol.SettingsErrorTimeout,
			Message:   "settings adapter timed out",
			Retryable: true,
		})
	}
}

func operationMethod(operation string) (string, bool) {
	switch settingsprotocol.SettingsOperation(operation) {
	case settingsprotocol.SettingsOperationLoad:
		return http.MethodGet, true
	case settingsprotocol.SettingsOperationValidate,
		settingsprotocol.SettingsOperationApply,
		settingsprotocol.SettingsOperationReset,
		settingsprotocol.SettingsOperationRestoreDefaults,
		settingsprotocol.SettingsOperationKeep,
		settingsprotocol.SettingsOperationRevert:
		return http.MethodPost, true
	default:
		return "", false
	}
}

func moduleOperationPath(operation string) string {
	switch settingsprotocol.SettingsOperation(operation) {
	case settingsprotocol.SettingsOperationLoad:
		return "state"
	case settingsprotocol.SettingsOperationRestoreDefaults:
		return "defaults"
	default:
		return operation
	}
}

func contextWithTimeout(request *http.Request, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		timeout = settingsregistry.DefaultAdapterTimeout
	}
	return context.WithTimeout(request.Context(), timeout)
}

func sameOrigin(request *http.Request) bool {
	origin := strings.TrimSpace(request.Header.Get("Origin"))
	if origin == "" {
		return true
	}
	parsed, err := url.Parse(origin)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return false
	}
	return strings.EqualFold(parsed.Host, request.Host)
}

func jsonContentType(value string) bool {
	value = strings.ToLower(strings.TrimSpace(strings.Split(value, ";")[0]))
	return value == "application/json"
}

type bufferedResponse struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func newBufferedResponse() *bufferedResponse {
	return &bufferedResponse{header: make(http.Header), status: http.StatusOK}
}

func (response *bufferedResponse) Header() http.Header { return response.header }

func (response *bufferedResponse) WriteHeader(status int) { response.status = status }

func (response *bufferedResponse) Write(data []byte) (int, error) { return response.body.Write(data) }

func (response *bufferedResponse) copyTo(target http.ResponseWriter) {
	for name, values := range response.header {
		for _, value := range values {
			target.Header().Add(name, value)
		}
	}
	target.WriteHeader(response.status)
	_, _ = target.Write(response.body.Bytes())
}

func writeMethodError(response http.ResponseWriter) {
	writeError(response, http.StatusMethodNotAllowed, settingsprotocol.SettingsError{
		Code:      settingsprotocol.SettingsErrorInvalidRequest,
		Message:   "method not allowed",
		Retryable: false,
	})
}

func writeError(response http.ResponseWriter, status int, settingsError settingsprotocol.SettingsError) {
	writeJSON(response, status, settingsError)
}

func writeJSON(response http.ResponseWriter, status int, payload any) {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(payload)
}
