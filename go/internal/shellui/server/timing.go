package server

import (
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const timingRecentSampleLimit = 128

type timingRecorder struct {
	mu      sync.Mutex
	buckets map[string]*timingBucket
	config  timingConfig
}

type timingConfig struct {
	UseCompositorctl bool
}

type timingBucket struct {
	Name       string
	Route      string
	Method     string
	Category   string
	Backend    string
	Count      uint64
	Total      time.Duration
	Min        time.Duration
	Max        time.Duration
	Recent     []time.Duration
	StatusCode map[int]uint64
}

type timingSummaryResponse struct {
	Schema                string              `json:"schema"`
	GeneratedAtUnixMillis int64               `json:"generatedAtUnixMillis"`
	WindowSampleLimit     int                 `json:"windowSampleLimit"`
	Routes                []timingSummaryView `json:"routes"`
}

type timingSummaryView struct {
	Name          string         `json:"name"`
	Route         string         `json:"route"`
	Method        string         `json:"method"`
	Category      string         `json:"category"`
	Backend       string         `json:"backend"`
	Count         uint64         `json:"count"`
	AverageMs     float64        `json:"averageMs"`
	MinMs         float64        `json:"minMs"`
	P50Ms         float64        `json:"p50Ms"`
	P95Ms         float64        `json:"p95Ms"`
	MaxMs         float64        `json:"maxMs"`
	StatusClasses map[string]int `json:"statusClasses"`
}

type statusCapturingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func newTimingRecorder(config timingConfig) *timingRecorder {
	return &timingRecorder{
		buckets: map[string]*timingBucket{},
		config:  config,
	}
}

func (recorder *timingRecorder) instrument(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		started := time.Now()
		capturing := &statusCapturingResponseWriter{ResponseWriter: response, statusCode: http.StatusOK}
		next.ServeHTTP(capturing, request)
		recorder.observe(request.Method, request.URL.Path, capturing.statusCode, time.Since(started))
	})
}

func (recorder *timingRecorder) observe(method string, path string, statusCode int, elapsed time.Duration) {
	classification := recorder.classify(method, path)
	if classification.Name == "" {
		return
	}
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	bucket := recorder.buckets[classification.Name]
	if bucket == nil {
		bucket = &timingBucket{
			Name:       classification.Name,
			Route:      classification.Route,
			Method:     method,
			Category:   classification.Category,
			Backend:    classification.Backend,
			StatusCode: map[int]uint64{},
		}
		recorder.buckets[classification.Name] = bucket
	}
	bucket.Count++
	bucket.Total += elapsed
	if bucket.Min == 0 || elapsed < bucket.Min {
		bucket.Min = elapsed
	}
	if elapsed > bucket.Max {
		bucket.Max = elapsed
	}
	bucket.Recent = append(bucket.Recent, elapsed)
	if len(bucket.Recent) > timingRecentSampleLimit {
		copy(bucket.Recent, bucket.Recent[len(bucket.Recent)-timingRecentSampleLimit:])
		bucket.Recent = bucket.Recent[:timingRecentSampleLimit]
	}
	bucket.StatusCode[statusCode]++
}

func (recorder *timingRecorder) summary(now time.Time) timingSummaryResponse {
	recorder.mu.Lock()
	defer recorder.mu.Unlock()

	routes := make([]timingSummaryView, 0, len(recorder.buckets))
	for _, bucket := range recorder.buckets {
		routes = append(routes, bucket.view())
	}
	sort.Slice(routes, func(i int, j int) bool {
		return routes[i].Name < routes[j].Name
	})
	return timingSummaryResponse{
		Schema:                "agora-de.shell-timing.v1",
		GeneratedAtUnixMillis: now.UnixMilli(),
		WindowSampleLimit:     timingRecentSampleLimit,
		Routes:                routes,
	}
}

func (bucket *timingBucket) view() timingSummaryView {
	recent := append([]time.Duration(nil), bucket.Recent...)
	sort.Slice(recent, func(i int, j int) bool {
		return recent[i] < recent[j]
	})
	statusClasses := map[string]int{}
	for statusCode, count := range bucket.StatusCode {
		class := statusClass(statusCode)
		statusClasses[class] += int(count)
	}
	var average time.Duration
	if bucket.Count > 0 {
		average = time.Duration(int64(bucket.Total) / int64(bucket.Count))
	}
	return timingSummaryView{
		Name:          bucket.Name,
		Route:         bucket.Route,
		Method:        bucket.Method,
		Category:      bucket.Category,
		Backend:       bucket.Backend,
		Count:         bucket.Count,
		AverageMs:     durationMillis(average),
		MinMs:         durationMillis(bucket.Min),
		P50Ms:         durationMillis(percentileDuration(recent, 50)),
		P95Ms:         durationMillis(percentileDuration(recent, 95)),
		MaxMs:         durationMillis(bucket.Max),
		StatusClasses: statusClasses,
	}
}

type timingClassification struct {
	Name     string
	Route    string
	Category string
	Backend  string
}

func (recorder *timingRecorder) classify(method string, path string) timingClassification {
	normalized := normalizeTimingPath(path)
	category := "shell_http"
	if method != http.MethodGet && method != http.MethodHead {
		category = "shell_action"
	}
	backend := "shell"
	switch normalized {
	case catalogrouteAppsPath:
		return timingClassification{Name: method + " " + normalized, Route: normalized, Category: category, Backend: "catalog"}
	case catalogrouteLaunchPath:
		return timingClassification{Name: method + " " + normalized, Route: normalized, Category: "launch_action", Backend: "native_launch"}
	case surfacerouteSurfacesPath:
		backend = backendForCompositor(recorder.config.UseCompositorctl)
	case LayoutPath:
		backend = backendForCompositor(recorder.config.UseCompositorctl)
	case LayoutActionPath, SurfaceActionPath, WorkspaceActionPath:
		backend = "compositorctl"
		category = "compositor_action"
	case WorkspacesPath:
		backend = backendForCompositor(recorder.config.UseCompositorctl)
	case WorkControlsPath:
		backend = backendForCompositor(recorder.config.UseCompositorctl)
	case OperatorStatusPath:
		backend = "operator"
	case ThemePath:
		backend = "theme"
	case TimingDiagnosticsPath:
		backend = "diagnostics"
	}
	if strings.HasPrefix(normalized, "/shell/dist/") {
		return timingClassification{Name: method + " /shell/dist/", Route: "/shell/dist/", Category: "shell_asset", Backend: "shell"}
	}
	if strings.HasPrefix(normalized, CatalogIconPathPrefix) {
		return timingClassification{Name: method + " " + CatalogIconPathPrefix, Route: CatalogIconPathPrefix, Category: "shell_asset", Backend: "catalog"}
	}
	if !isKnownTimingPath(normalized) {
		return timingClassification{Name: method + " other", Route: "other", Category: category, Backend: "shell"}
	}
	return timingClassification{Name: method + " " + normalized, Route: normalized, Category: category, Backend: backend}
}

func normalizeTimingPath(path string) string {
	if path == "" {
		return "/"
	}
	return path
}

func isKnownTimingPath(path string) bool {
	switch path {
	case catalogrouteAppsPath, catalogrouteLaunchPath, surfacerouteSurfacesPath, LayoutPath, LayoutActionPath,
		SurfaceActionPath, OperatorStatusPath, ThemePath, WorkspacesPath, WorkspaceActionPath, WorkControlsPath, TimingDiagnosticsPath:
		return true
	default:
		return false
	}
}

func backendForCompositor(useCompositorctl bool) string {
	if useCompositorctl {
		return "compositorctl"
	}
	return "fixture"
}

func percentileDuration(ordered []time.Duration, percentile int) time.Duration {
	if len(ordered) == 0 {
		return 0
	}
	if len(ordered) == 1 {
		return ordered[0]
	}
	rank := (float64(percentile) / 100) * float64(len(ordered)-1)
	lower := int(math.Floor(rank))
	upper := int(math.Ceil(rank))
	if lower == upper {
		return ordered[lower]
	}
	lowerWeight := float64(upper) - rank
	upperWeight := rank - float64(lower)
	return time.Duration(float64(ordered[lower])*lowerWeight + float64(ordered[upper])*upperWeight)
}

func durationMillis(value time.Duration) float64 {
	return math.Round(float64(value.Microseconds())/10) / 100
}

func statusClass(statusCode int) string {
	if statusCode <= 0 {
		return "unknown"
	}
	return strconv.Itoa(statusCode/100) + "xx"
}

func (writer *statusCapturingResponseWriter) WriteHeader(statusCode int) {
	writer.statusCode = statusCode
	writer.ResponseWriter.WriteHeader(statusCode)
}

const (
	catalogrouteAppsPath     = "/api/catalog/apps"
	catalogrouteLaunchPath   = "/api/catalog/launch"
	surfacerouteSurfacesPath = "/api/surfaces"
)
