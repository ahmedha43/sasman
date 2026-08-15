package ota

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type DockerPullResult struct {
	Arch      string  `json:"arch"`
	Platform  string  `json:"platform"`
	TarFile   string  `json:"tar_file"`
	TarSizeMB float64 `json:"tar_size_mb"`
	Sha256    string  `json:"sha256"`
	Success   bool    `json:"success"`
	Error     string  `json:"error,omitempty"`
}

type DockerPullSummary struct {
	Image       string             `json:"image"`
	Version     string             `json:"version"`
	Channel     string             `json:"channel"`
	Results     []DockerPullResult `json:"results"`
	TotalPulled int                `json:"total_pulled"`
	Errors      []string           `json:"errors,omitempty"`
}

var supportedPlatforms = []struct {
	TargetArch string
	Platform   string
	OutputTar  string
}{
	{TargetArch: "linux_arm64", Platform: "linux/arm64", OutputTar: "sasman-arm64.tar"},
	{TargetArch: "linux_arm", Platform: "linux/arm/v7", OutputTar: "sasman-armv7.tar"},
	{TargetArch: "linux_amd64", Platform: "linux/amd64", OutputTar: "sasman-amd64.tar"},
}

func (m *Manager) PullAndPublishFromDocker(ctx context.Context, image string, version string, channel string, releaseNotes string) (*DockerPullSummary, error) {
	image = strings.TrimSpace(image)
	if image == "" {
		return nil, fmt.Errorf("docker image name is required (e.g. ahmedkin99/sasman-manager:v5)")
	}
	if version == "" {
		version = "5.1.0"
	}
	if channel == "" {
		channel = "stable"
	}

	// Ensure releases storage directory exists
	dataDir := os.Getenv("SASMAN_DATA_DIR")
	if dataDir == "" {
		dataDir = "/app/data"
	}
	releasesDir := filepath.Join(dataDir, "releases")
	_ = os.MkdirAll(releasesDir, 0755)

	summary := &DockerPullSummary{
		Image:   image,
		Version: version,
		Channel: channel,
		Results: make([]DockerPullResult, 0, len(supportedPlatforms)),
	}

	for _, target := range supportedPlatforms {
		log.Printf("[OTA Docker Pull] 🚀 Pulling %s for platform %s...", image, target.Platform)
		res := DockerPullResult{
			Arch:     target.TargetArch,
			Platform: target.Platform,
			TarFile:  target.OutputTar,
		}

		// 1. docker pull --platform <platform> <image>
		pullCmd := exec.CommandContext(ctx, "docker", "pull", "--platform", target.Platform, image)
		pullOut, err := pullCmd.CombinedOutput()
		if err != nil {
			res.Success = false
			res.Error = fmt.Sprintf("docker pull failed: %v (%s)", err, strings.TrimSpace(string(pullOut)))
			log.Printf("[OTA Docker Pull] ❌ %s: %s", target.Platform, res.Error)
			summary.Results = append(summary.Results, res)
			summary.Errors = append(summary.Errors, res.Error)
			continue
		}

		// 2. Save full image TAR for MikroTik Container Import (/download/sasman-*.tar)
		tarDest := filepath.Join(releasesDir, target.OutputTar)
		saveCmd := exec.CommandContext(ctx, "docker", "save", image, "-o", tarDest)
		saveOut, err := saveCmd.CombinedOutput()
		if err != nil {
			log.Printf("[OTA Docker Pull] ⚠️ docker save failed: %v (%s)", err, strings.TrimSpace(string(saveOut)))
		} else {
			if fi, err := os.Stat(tarDest); err == nil {
				res.TarSizeMB = float64(fi.Size()) / (1024 * 1024)
				log.Printf("[OTA Docker Pull] 💾 Saved %s (%.1f MB)", target.OutputTar, res.TarSizeMB)
			}
		}

		// 3. Extract agent binary for in-place OTA update
		createCmd := exec.CommandContext(ctx, "docker", "create", "--platform", target.Platform, image)
		createOut, err := createCmd.CombinedOutput()
		if err != nil {
			res.Success = false
			res.Error = fmt.Sprintf("docker create failed: %v", err)
			summary.Results = append(summary.Results, res)
			summary.Errors = append(summary.Errors, res.Error)
			continue
		}
		cid := strings.TrimSpace(string(createOut))

		// docker cp <cid>:/app/main -
		var binData []byte
		binPaths := []string{"/app/main", "/app/sasman-agent", "/main"}
		for _, bPath := range binPaths {
			cpCmd := exec.CommandContext(ctx, "docker", "cp", cid+":"+bPath, "-")
			tarOut, err := cpCmd.Output()
			if err == nil && len(tarOut) > 0 {
				extracted, extErr := extractSingleFileFromTar(tarOut)
				if extErr == nil && len(extracted) > 0 {
					binData = extracted
					break
				}
			}
		}

		// Clean up temporary container
		_ = exec.Command("docker", "rm", "-f", cid).Run()

		if len(binData) == 0 {
			res.Success = false
			res.Error = "could not find binary /app/main inside image"
			summary.Results = append(summary.Results, res)
			summary.Errors = append(summary.Errors, res.Error)
			continue
		}

		// 4. Sign and publish release to SQLite database
		manifest, err := m.PublishRelease(version, channel, target.TargetArch, releaseNotes, binData)
		if err != nil {
			res.Success = false
			res.Error = fmt.Sprintf("publish release failed: %v", err)
			summary.Results = append(summary.Results, res)
			summary.Errors = append(summary.Errors, res.Error)
			continue
		}

		res.Sha256 = manifest.Sha256
		res.Success = true
		summary.Results = append(summary.Results, res)
		summary.TotalPulled++
		log.Printf("[OTA Docker Pull] ✅ Successfully processed %s (%s) - SHA256: %s", target.TargetArch, version, manifest.Sha256)
	}

	if summary.TotalPulled == 0 && len(summary.Errors) > 0 {
		return summary, fmt.Errorf("failed to pull image for any architecture: %s", strings.Join(summary.Errors, "; "))
	}

	return summary, nil
}

func extractSingleFileFromTar(tarBytes []byte) ([]byte, error) {
	tr := tar.NewReader(bytes.NewReader(tarBytes))
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if header.Typeflag == tar.TypeReg {
			var buf bytes.Buffer
			if _, err := io.Copy(&buf, tr); err != nil {
				return nil, err
			}
			return buf.Bytes(), nil
		}
	}
	return nil, fmt.Errorf("no regular file found in tar stream")
}
