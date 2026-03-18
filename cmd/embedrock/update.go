package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// updateHTTPClient is used for all HTTP requests during self-update.
var updateHTTPClient = &http.Client{Timeout: 30 * time.Second}

const defaultGitHubAPIBase = "https://api.github.com"

// ghRelease is the GitHub API response for a release.
type ghRelease struct {
	TagName string    `json:"tag_name"`
	Assets  []ghAsset `json:"assets"`
}

// ghAsset is a single asset in a GitHub release.
type ghAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// runUpdate performs the self-update. apiBase allows tests to inject a mock server URL.
func runUpdate(currentVersion, apiBase string) error {
	return runUpdateTo(currentVersion, apiBase, "")
}

// runUpdateTo performs the self-update with an optional execPath override for testing.
func runUpdateTo(currentVersion, apiBase, execPathOverride string) error {
	if apiBase == "" {
		apiBase = defaultGitHubAPIBase
	}

	// 1. Fetch latest release metadata
	releaseURL := apiBase + "/repos/inceptionstack/embedrock/releases/latest"
	resp, err := updateHTTPClient.Get(releaseURL)
	if err != nil {
		return fmt.Errorf("failed to check for updates: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return fmt.Errorf("failed to parse release info: %w", err)
	}

	latestVersion := release.TagName

	// 2. Compare versions
	if currentVersion != "dev" && currentVersion == latestVersion {
		fmt.Printf("embedrock %s is already up to date\n", currentVersion)
		return nil
	}

	// 3. Find the right asset for this OS/arch
	assetName := fmt.Sprintf("embedrock-%s-%s", runtime.GOOS, runtime.GOARCH)
	var binaryAsset *ghAsset
	var checksumAsset *ghAsset

	for i := range release.Assets {
		switch release.Assets[i].Name {
		case assetName:
			binaryAsset = &release.Assets[i]
		case "checksums.txt":
			checksumAsset = &release.Assets[i]
		}
	}

	if binaryAsset == nil {
		return fmt.Errorf("no binary available for %s/%s (looked for %s)", runtime.GOOS, runtime.GOARCH, assetName)
	}

	// 4. Download the binary to a temp file
	binResp, err := updateHTTPClient.Get(binaryAsset.BrowserDownloadURL)
	if err != nil {
		return fmt.Errorf("failed to download binary: %w", err)
	}
	defer binResp.Body.Close()

	if binResp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download binary: HTTP %d", binResp.StatusCode)
	}

	var execPath string
	if execPathOverride != "" {
		execPath = execPathOverride
	} else {
		execPath, err = os.Executable()
		if err != nil {
			return fmt.Errorf("failed to determine executable path: %w", err)
		}
		execPath, err = filepath.EvalSymlinks(execPath)
		if err != nil {
			return fmt.Errorf("failed to resolve executable path: %w", err)
		}
	}

	tmpFile, err := os.CreateTemp(filepath.Dir(execPath), ".embedrock-update-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	success := false
	defer func() {
		if !success {
			os.Remove(tmpPath)
		}
	}()

	hasher := sha256.New()
	writer := io.MultiWriter(tmpFile, hasher)

	if _, err := io.Copy(writer, binResp.Body); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to write binary: %w", err)
	}
	tmpFile.Close()

	downloadedHash := hex.EncodeToString(hasher.Sum(nil))

	// 5. Verify checksum
	if checksumAsset != nil {
		expectedHash, err := fetchExpectedChecksum(checksumAsset.BrowserDownloadURL, assetName)
		if err != nil {
			return fmt.Errorf("failed to verify checksum: %w", err)
		}
		if downloadedHash != expectedHash {
			return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedHash, downloadedHash)
		}
	}

	// 6. Make executable
	if err := os.Chmod(tmpPath, 0755); err != nil {
		return fmt.Errorf("failed to set permissions: %w", err)
	}

	// 7. Rename over the current binary (atomic on same filesystem)
	if err := os.Rename(tmpPath, execPath); err != nil {
		return fmt.Errorf("failed to replace binary: %w", err)
	}

	// Rename succeeded, skip deferred cleanup
	success = true

	if currentVersion == "dev" {
		fmt.Printf("Updated embedrock to %s\n", latestVersion)
	} else {
		fmt.Printf("Updated embedrock from %s to %s\n", currentVersion, latestVersion)
	}

	// Try to restart if running as a systemd service
	tryRestartService()

	return nil
}

// tryRestartService checks if embedrock is running as a systemd service and restarts it.
// Never fails the update — only prints warnings.
func tryRestartService() {
	// Check if systemctl exists
	systemctl, err := exec.LookPath("systemctl")
	if err != nil {
		fmt.Println("Restart embedrock manually to use the new version")
		return
	}

	// Check if the service is active
	cmd := exec.Command(systemctl, "is-active", "embedrock.service")
	if err := cmd.Run(); err != nil {
		// Service not active or doesn't exist
		fmt.Println("Restart embedrock manually to use the new version")
		return
	}

	// Only attempt restart if running as root
	if os.Geteuid() != 0 {
		fmt.Println("Restart embedrock.service manually (requires sudo)")
		return
	}

	// Restart the service (already root, no sudo needed)
	cmd = exec.Command(systemctl, "restart", "embedrock")
	if output, err := cmd.CombinedOutput(); err != nil {
		fmt.Printf("Warning: failed to restart embedrock.service: %s\n", strings.TrimSpace(string(output)))
		fmt.Println("Restart embedrock manually to use the new version")
		return
	}

	fmt.Println("Restarted embedrock.service")
}

// fetchExpectedChecksum downloads checksums.txt and returns the SHA-256 hash for the given asset.
func fetchExpectedChecksum(url, assetName string) (string, error) {
	resp, err := updateHTTPClient.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d fetching checksums", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	for _, line := range strings.Split(strings.TrimSpace(string(body)), "\n") {
		// Format: <hash>  <filename> (two spaces, matching sha256sum output)
		parts := strings.Fields(line)
		if len(parts) == 2 && parts[1] == assetName {
			return parts[0], nil
		}
	}

	return "", fmt.Errorf("no checksum found for %s", assetName)
}
