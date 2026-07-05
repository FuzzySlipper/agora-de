package compositorbridge

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func (bridge *Bridge) capturePhysicalOutput(request CaptureOutputRequest, output LogicalOutput) (CaptureSurfaceResponse, error) {
	bridge.mu.Lock()
	bridge.captureSeq++
	requestID := fmt.Sprintf("output-capture-%d-%d", time.Now().UnixNano(), bridge.captureSeq)
	bridge.mu.Unlock()

	tmpDir, err := os.MkdirTemp("", "agora-output-capture-")
	if err != nil {
		return CaptureSurfaceResponse{}, err
	}
	defer os.RemoveAll(tmpDir)
	if err := preparePhysicalCaptureTempDir(tmpDir); err != nil {
		return CaptureSurfaceResponse{}, err
	}

	tmpPath := filepath.Join(tmpDir, sanitizeArtifactID(request.Name)+".png")
	if err := runPhysicalOutputCapture(request.Name, tmpPath); err != nil {
		return CaptureSurfaceResponse{}, err
	}
	captureBytes, err := os.ReadFile(tmpPath)
	if err != nil {
		return CaptureSurfaceResponse{}, fmt.Errorf("read output capture: %w", err)
	}
	visualInspection, err := inspectPNG(captureBytes)
	if err != nil {
		return CaptureSurfaceResponse{}, fmt.Errorf("inspect output capture png: %w", err)
	}

	width, height := output.Width, output.Height
	if visualInspection != nil {
		width = visualInspection.Width
		height = visualInspection.Height
	}
	sessionID := request.SessionID
	dir := captureRoot()
	if request.Export {
		if sessionID == "" {
			sessionID = "unscoped"
		}
		dir = filepath.Join(artifactRoot(), sessionID, requestID)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return CaptureSurfaceResponse{}, err
	}
	path := filepath.Join(dir, requestID+".png")
	if err := os.WriteFile(path, captureBytes, 0o644); err != nil {
		return CaptureSurfaceResponse{}, err
	}
	sha := sha256Hex(captureBytes)
	capturedAt := time.Now()
	surfaceID := "output:" + request.Name
	response := CaptureSurfaceResponse{
		SurfaceID:        surfaceID,
		RequestID:        requestID,
		Path:             path,
		ImagePath:        path,
		Width:            uint32(width),
		Height:           uint32(height),
		Format:           "png",
		SHA256:           sha,
		CapturedAt:       capturedAt,
		VisualInspection: visualInspection,
	}
	if request.Export {
		evidenceClass := request.EvidenceClass
		if evidenceClass == "" {
			evidenceClass = "viewport_screenshot"
		}
		artifact := ArtifactRecord{
			ArtifactID:            requestID,
			SessionID:             sessionID,
			SurfaceID:             surfaceID,
			RequestID:             requestID,
			ImagePath:             path,
			IndexPath:             filepath.Join(dir, "index.json"),
			Width:                 uint32(width),
			Height:                uint32(height),
			Format:                "png",
			SHA256:                sha,
			CaptureBackend:        "physical_output_grim",
			AuditCorrelationID:    request.AuditCorrelationID,
			EvidenceClass:         evidenceClass,
			Timestamp:             capturedAt,
			ASHACommandSequenceID: request.ASHACommandSequenceID,
			VisualInspection:      visualInspection,
		}
		if visualInspection != nil && visualInspection.Status == "blank" {
			artifact.Warnings = append(artifact.Warnings, "capture payload classified as "+visualInspection.Classification)
		}
		if sessionID == "unscoped" {
			artifact.Warnings = append(artifact.Warnings, "capture was not associated with a compositor session")
		}
		indexBytes, _ := json.MarshalIndent(artifact, "", "  ")
		if err := os.WriteFile(artifact.IndexPath, indexBytes, 0o644); err != nil {
			return CaptureSurfaceResponse{}, err
		}
		response.Artifact = &artifact
	}
	return response, nil
}

func runPhysicalOutputCapture(outputName, path string) error {
	grim := physicalCaptureBinary()
	if grim == "" {
		return classifiedError{class: ErrorBackendUnsupported, message: "grim output capture backend is not installed"}
	}
	args := []string{"-o", outputName, path}
	cmd := exec.Command(grim, args...)
	captureUser, userConfigured := os.LookupEnv("AGORA_OUTPUT_CAPTURE_USER")
	if !userConfigured && os.Geteuid() == 0 {
		captureUser = "agent"
	}
	if captureUser != "" {
		envArgs := []string{"-u", captureUser, "--", "env"}
		envArgs = append(envArgs, physicalCaptureEnv()...)
		envArgs = append(envArgs, grim)
		envArgs = append(envArgs, args...)
		cmd = exec.Command("runuser", envArgs...)
	} else {
		cmd.Env = append(os.Environ(), physicalCaptureEnv()...)
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			message = err.Error()
		}
		return classifiedError{class: ErrorCaptureDenied, message: "physical output capture failed: " + message}
	}
	return nil
}

func preparePhysicalCaptureTempDir(path string) error {
	captureUser, userConfigured := os.LookupEnv("AGORA_OUTPUT_CAPTURE_USER")
	if !userConfigured && os.Geteuid() == 0 {
		captureUser = "agent"
	}
	if captureUser == "" {
		return nil
	}
	u, err := user.Lookup(captureUser)
	if err != nil {
		return classifiedError{class: ErrorCaptureDenied, message: fmt.Sprintf("lookup capture user %q: %v", captureUser, err)}
	}
	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		return classifiedError{class: ErrorCaptureDenied, message: fmt.Sprintf("parse capture user uid %q: %v", u.Uid, err)}
	}
	gid, err := strconv.Atoi(u.Gid)
	if err != nil {
		return classifiedError{class: ErrorCaptureDenied, message: fmt.Sprintf("parse capture user gid %q: %v", u.Gid, err)}
	}
	if err := os.Chown(path, uid, gid); err != nil {
		return err
	}
	return os.Chmod(path, 0o700)
}

func physicalCaptureBinary() string {
	if configured := os.Getenv("AGORA_OUTPUT_CAPTURE_GRIM"); configured != "" {
		if _, err := os.Stat(configured); err == nil {
			return configured
		}
		return ""
	}
	if _, err := os.Stat("/usr/bin/grim"); err == nil {
		return "/usr/bin/grim"
	}
	path, err := exec.LookPath("grim")
	if err != nil {
		return ""
	}
	return path
}

func physicalCaptureEnv() []string {
	runtimeDir := os.Getenv("AGORA_OUTPUT_CAPTURE_RUNTIME_DIR")
	if runtimeDir == "" {
		runtimeDir = "/run/user/1001"
	}
	display := os.Getenv("AGORA_OUTPUT_CAPTURE_WAYLAND_DISPLAY")
	if display == "" {
		display = "wayland-1"
	}
	return []string{"XDG_RUNTIME_DIR=" + runtimeDir, "WAYLAND_DISPLAY=" + display}
}

func captureRoot() string {
	if root := os.Getenv("AGORA_CAPTURE_ROOT"); root != "" {
		return root
	}
	return "/run/agent-os/captures"
}

func artifactRoot() string {
	if root := os.Getenv("AGORA_ARTIFACT_ROOT"); root != "" {
		return root
	}
	return "/run/agent-os/artifacts"
}

func sanitizeArtifactID(id string) string {
	replacer := strings.NewReplacer("/", "-", "\\", "-", ":", "-", " ", "-", "\t", "-", "\n", "-")
	sanitized := strings.Trim(replacer.Replace(id), "-")
	if sanitized == "" {
		return "artifact"
	}
	return sanitized
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
