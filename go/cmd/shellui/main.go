package main

import (
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"strings"

	"agora-de.local/go/internal/shellui/server"
)

func main() {
	listen := flag.String("listen", env("AGORA_DE_SHELLUI_LISTEN", server.DefaultListenAddress), "HTTP listen address")
	staticRoot := flag.String("static-root", os.Getenv("AGORA_DE_SHELLUI_STATIC_ROOT"), "optional shell static asset root")
	fixtureProviders := flag.Bool("fixture-providers", envBool("AGORA_DE_SHELLUI_FIXTURE_PROVIDERS", true), "serve deterministic deployment-testing providers")
	surfaceProvider := flag.String("surface-provider", env("AGORA_DE_SHELLUI_SURFACE_PROVIDER", server.SurfaceProviderFixture), "surface provider: fixture or compositorctl")
	compositorctlPath := flag.String("compositorctl", env("AGORA_DE_SHELLUI_COMPOSITORCTL", "compositorctl"), "compositorctl path for live surface provider")
	flag.Parse()

	handler, err := server.NewHandler(server.Config{
		StaticRoot:        *staticRoot,
		FixtureProviders:  *fixtureProviders,
		SurfaceProvider:   *surfaceProvider,
		CompositorctlPath: *compositorctlPath,
	})
	if err != nil {
		log.Fatal(err)
	}

	httpServer := &http.Server{
		Addr:              *listen,
		Handler:           handler,
		ReadHeaderTimeout: 5_000_000_000,
	}
	log.Printf("agora-de shellui listening on http://%s", *listen)
	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

func env(name string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	return value
}

func envBool(name string, fallback bool) bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv(name)))
	if value == "" {
		return fallback
	}
	return value == "1" || value == "true" || value == "yes"
}
