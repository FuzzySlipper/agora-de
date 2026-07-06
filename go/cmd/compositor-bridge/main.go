package main

import (
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"

	"agora-de.local/go/internal/compositorbridge"
)

const (
	defaultSocketDir     = "/run/agent-os"
	defaultPluginSocket  = "/run/agent-os/compositor-bridge.sock"
	defaultControlSocket = "/run/agent-os/compositor-control.sock"
)

func main() {
	socketDir := envString("AGORA_DE_COMPOSITOR_SOCKET_DIR", defaultSocketDir)
	pluginSocket := envString("AGORA_DE_COMPOSITOR_PLUGIN_SOCKET", defaultPluginSocket)
	controlSocket := envString("AGORA_DE_COMPOSITOR_CONTROL_SOCKET", defaultControlSocket)
	layoutSettingsPath := envString("AGORA_DE_LAYOUT_SETTINGS", defaultLayoutSettingsPath())
	compositorUID := envUint32("AGORA_COMPOSITOR_UID", 0)
	compositorGID := envInt("AGORA_COMPOSITOR_GID", int(compositorUID))

	if err := os.MkdirAll(socketDir, 0o775); err != nil {
		log.Fatalf("mkdir %s: %v", socketDir, err)
	}
	_ = os.Remove(pluginSocket)
	_ = os.Remove(controlSocket)

	pluginListener, err := net.Listen("unix", pluginSocket)
	if err != nil {
		log.Fatalf("listen plugin socket: %v", err)
	}
	defer pluginListener.Close()

	controlListener, err := net.Listen("unix", controlSocket)
	if err != nil {
		log.Fatalf("listen control socket: %v", err)
	}
	defer controlListener.Close()

	configureSocket(pluginSocket, 0o660, 0, compositorGID)
	configureSocket(controlSocket, 0o660, 0, compositorGID)

	bridge := compositorbridge.New(compositorbridge.Config{
		AllowedPluginUID:   compositorUID,
		LayoutSettingsPath: layoutSettingsPath,
	})

	log.Printf("agora-de compositor bridge plugin socket: %s", pluginSocket)
	log.Printf("agora-de compositor bridge control socket: %s", controlSocket)

	go shutdownOnSignal(pluginListener, controlListener)
	go acceptLoop(pluginListener, bridge.HandlePluginConn)
	acceptLoop(controlListener, bridge.HandleControlConn)
}

func acceptLoop(listener net.Listener, handle func(net.Conn)) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("accept: %v", err)
			return
		}
		go handle(conn)
	}
}

func shutdownOnSignal(listeners ...net.Listener) {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	log.Println("shutting down")
	for _, listener := range listeners {
		_ = listener.Close()
	}
	os.Exit(0)
}

func defaultLayoutSettingsPath() string {
	configDir, err := os.UserConfigDir()
	if err != nil || configDir == "" {
		return ""
	}
	return filepath.Join(configDir, "agora-de", "layout-settings.json")
}

func configureSocket(path string, mode os.FileMode, uid int, gid int) {
	if os.Geteuid() == 0 {
		if err := os.Chown(path, uid, gid); err != nil {
			log.Fatalf("chown %s: %v", path, err)
		}
	}
	if err := os.Chmod(path, mode); err != nil {
		log.Fatalf("chmod %s: %v", path, err)
	}
}

func envString(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func envUint32(name string, fallback uint32) uint32 {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseUint(value, 10, 32)
	if err != nil {
		log.Fatalf("parse %s: %v", name, err)
	}
	return uint32(parsed)
}

func envInt(name string, fallback int) int {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		log.Fatalf("parse %s: %v", name, err)
	}
	return parsed
}
