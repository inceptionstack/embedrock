package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

const unitFilePath = "/etc/systemd/system/embedrock.service"
const installBinPath = "/usr/local/bin/embedrock"

// generateUnitFile returns the systemd unit file content for the given configuration.
func generateUnitFile(port int, region, model string) string {
	return fmt.Sprintf(`[Unit]
Description=embedrock - OpenAI-compatible embedding proxy for Amazon Bedrock
After=network.target

[Service]
Type=simple
ExecStart=%s --port %d --region %s --model %s
Restart=always
RestartSec=5
Environment=HOME=/root

[Install]
WantedBy=multi-user.target
`, installBinPath, port, region, model)
}

// runInstallDaemon installs embedrock as a systemd service.
func runInstallDaemon(port int, region, model string) error {
	// Check for root
	if os.Geteuid() != 0 {
		return fmt.Errorf("install-daemon requires root. Run with sudo.")
	}

	// Check that systemctl is available
	systemctl, err := exec.LookPath("systemctl")
	if err != nil {
		return fmt.Errorf("systemctl not found — install-daemon requires systemd")
	}

	// 1. Copy current binary to /usr/local/bin/embedrock
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to determine executable path: %w", err)
	}
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return fmt.Errorf("failed to resolve executable path: %w", err)
	}

	if execPath != installBinPath {
		srcFile, err := os.Open(execPath)
		if err != nil {
			return fmt.Errorf("failed to read current binary: %w", err)
		}

		tmpFile, err := os.CreateTemp(filepath.Dir(installBinPath), ".embedrock-install-*")
		if err != nil {
			srcFile.Close()
			return fmt.Errorf("failed to create temp file: %w", err)
		}
		tmpPath := tmpFile.Name()

		if _, err := io.Copy(tmpFile, srcFile); err != nil {
			tmpFile.Close()
			srcFile.Close()
			os.Remove(tmpPath)
			return fmt.Errorf("failed to write binary: %w", err)
		}
		srcFile.Close()
		tmpFile.Close()

		if err := os.Chmod(tmpPath, 0755); err != nil {
			os.Remove(tmpPath)
			return fmt.Errorf("failed to set permissions: %w", err)
		}
		if err := os.Rename(tmpPath, installBinPath); err != nil {
			os.Remove(tmpPath)
			return fmt.Errorf("failed to install binary to %s: %w", installBinPath, err)
		}
		fmt.Printf("Installed binary to %s\n", installBinPath)
	} else {
		fmt.Printf("Binary already at %s\n", installBinPath)
	}

	// 2. Write the systemd unit file
	unitContent := generateUnitFile(port, region, model)
	if err := os.WriteFile(unitFilePath, []byte(unitContent), 0644); err != nil {
		return fmt.Errorf("failed to write unit file to %s: %w", unitFilePath, err)
	}
	fmt.Printf("Wrote %s\n", unitFilePath)

	// 3. Reload, enable, and start
	commands := []struct {
		args []string
		desc string
	}{
		{[]string{systemctl, "daemon-reload"}, "daemon-reload"},
		{[]string{systemctl, "enable", "embedrock"}, "enable"},
		{[]string{systemctl, "start", "embedrock"}, "start"},
	}

	for _, c := range commands {
		cmd := exec.Command(c.args[0], c.args[1:]...)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("systemctl %s failed: %s", c.desc, string(output))
		}
	}

	fmt.Println("embedrock.service enabled and started")

	// 4. Print status
	cmd := exec.Command(systemctl, "status", "embedrock", "--no-pager")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run() // ignore error — status exits non-zero for some states

	return nil
}
