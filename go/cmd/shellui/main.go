package main

import (
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"agora-de.local/go/internal/shellui/server"
)

func main() {
	listen := flag.String("listen", env("AGORA_DE_SHELLUI_LISTEN", server.DefaultListenAddress), "HTTP listen address")
	staticRoot := flag.String("static-root", os.Getenv("AGORA_DE_SHELLUI_STATIC_ROOT"), "optional shell static asset root")
	fixtureProviders := flag.Bool("fixture-providers", envBool("AGORA_DE_SHELLUI_FIXTURE_PROVIDERS", true), "serve deterministic deployment-testing providers")
	themeID := flag.String("theme-id", env("AGORA_DE_SHELLUI_THEME_ID", ""), "bundled shell theme id, empty for default")
	themeManifest := flag.String("theme-manifest", env("AGORA_DE_SHELLUI_THEME_MANIFEST", ""), "optional shell theme manifest path")
	catalogProvider := flag.String("catalog-provider", env("AGORA_DE_SHELLUI_CATALOG_PROVIDER", server.CatalogProviderFixture), "catalog provider: fixture or desktop_entries")
	desktopEntryRoots := flag.String("desktop-entry-roots", env("AGORA_DE_SHELLUI_DESKTOP_ENTRY_ROOTS", ""), "desktop entry roots for desktop_entries catalog provider")
	iconThemeRoots := flag.String("icon-theme-roots", env("AGORA_DE_SHELLUI_ICON_THEME_ROOTS", ""), "optional icon theme roots for desktop entry icon resolution")
	iconPixmapRoots := flag.String("icon-pixmap-roots", env("AGORA_DE_SHELLUI_ICON_PIXMAP_ROOTS", ""), "optional pixmap roots for desktop entry icon resolution")
	surfaceProvider := flag.String("surface-provider", env("AGORA_DE_SHELLUI_SURFACE_PROVIDER", server.SurfaceProviderFixture), "surface provider: fixture or compositorctl")
	compositorctlPath := flag.String("compositorctl", env("AGORA_DE_SHELLUI_COMPOSITORCTL", "compositorctl"), "compositorctl path for live surface provider")
	systemctlPath := flag.String("systemctl", env("AGORA_DE_SHELLUI_SYSTEMCTL", "systemctl"), "systemctl path for shell settings actions")
	nativeLaunchProvider := flag.String("native-launch-provider", env("AGORA_DE_SHELLUI_NATIVE_LAUNCH_PROVIDER", server.NativeLaunchProviderDisabled), "native launch provider: disabled or structured_compositorctl")
	nativeLaunchAllowlist := flag.String("native-launch-allowlist", env("AGORA_DE_SHELLUI_NATIVE_LAUNCH_ALLOWLIST", ""), "comma-separated desktop entry ids allowed for native launch")
	nativeLaunchUID := flag.Int("native-launch-uid", envInt("AGORA_DE_SHELLUI_NATIVE_LAUNCH_UID", 0), "requester uid for native launch")
	nativeLaunchGID := flag.Int("native-launch-gid", envInt("AGORA_DE_SHELLUI_NATIVE_LAUNCH_GID", 0), "requester gid for native launch")
	nativeLaunchSessionToken := flag.String("native-launch-session-token", env("AGORA_DE_SHELLUI_NATIVE_LAUNCH_SESSION_TOKEN", ""), "session token for native launch")
	nativeLaunchOutput := flag.String("native-launch-output", env("AGORA_DE_SHELLUI_NATIVE_LAUNCH_OUTPUT", ""), "output name for native launch placement")
	nativeLaunchHome := flag.String("native-launch-home", env("AGORA_DE_SHELLUI_NATIVE_LAUNCH_HOME", os.Getenv("HOME")), "home directory used as native launch cwd default")
	nativeLaunchWorkingDir := flag.String("native-launch-working-dir", env("AGORA_DE_SHELLUI_NATIVE_LAUNCH_WORKING_DIR", ""), "explicit native launch working directory")
	flag.Parse()

	handler, err := server.NewHandler(server.Config{
		StaticRoot:               *staticRoot,
		FixtureProviders:         *fixtureProviders,
		ThemeID:                  *themeID,
		ThemeManifestPath:        *themeManifest,
		CatalogProvider:          *catalogProvider,
		DesktopEntryRoots:        splitPathList(*desktopEntryRoots),
		IconThemeRoots:           splitPathList(*iconThemeRoots),
		IconPixmapRoots:          splitPathList(*iconPixmapRoots),
		SurfaceProvider:          *surfaceProvider,
		CompositorctlPath:        *compositorctlPath,
		SystemctlPath:            *systemctlPath,
		NativeLaunchProvider:     *nativeLaunchProvider,
		NativeLaunchAllowlist:    splitCSV(*nativeLaunchAllowlist),
		NativeLaunchRequesterUID: *nativeLaunchUID,
		NativeLaunchRequesterGID: *nativeLaunchGID,
		NativeLaunchSessionToken: *nativeLaunchSessionToken,
		NativeLaunchOutputName:   *nativeLaunchOutput,
		NativeLaunchHome:         *nativeLaunchHome,
		NativeLaunchWorkingDir:   *nativeLaunchWorkingDir,
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

func envInt(name string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func splitPathList(value string) []string {
	items := filepath.SplitList(value)
	roots := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item != "" {
			roots = append(roots, item)
		}
	}
	return roots
}

func splitCSV(value string) []string {
	items := strings.Split(value, ",")
	values := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item != "" {
			values = append(values, item)
		}
	}
	return values
}
