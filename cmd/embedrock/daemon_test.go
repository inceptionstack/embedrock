package main

import (
	"os"
	"strings"
	"testing"
)

func TestGenerateUnitFile(t *testing.T) {
	tests := []struct {
		name   string
		port   int
		region string
		model  string
		checks []string
	}{
		{
			name:   "default settings",
			port:   8089,
			region: "us-east-1",
			model:  "amazon.titan-embed-text-v2:0",
			checks: []string{
				"ExecStart=/usr/local/bin/embedrock --port 8089 --region us-east-1 --model amazon.titan-embed-text-v2:0",
				"[Unit]",
				"[Service]",
				"[Install]",
				"Type=simple",
				"Restart=always",
				"RestartSec=5",
				"Environment=HOME=/root",
				"After=network.target",
				"WantedBy=multi-user.target",
				"Description=embedrock",
			},
		},
		{
			name:   "custom port and model",
			port:   9090,
			region: "eu-west-1",
			model:  "cohere.embed-v4:0",
			checks: []string{
				"ExecStart=/usr/local/bin/embedrock --port 9090 --region eu-west-1 --model cohere.embed-v4:0",
			},
		},
		{
			name:   "high port number",
			port:   443,
			region: "ap-southeast-1",
			model:  "amazon.titan-embed-text-v2:0",
			checks: []string{
				"--port 443",
				"--region ap-southeast-1",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := generateUnitFile(tc.port, tc.region, tc.model)

			for _, check := range tc.checks {
				if !strings.Contains(result, check) {
					t.Errorf("unit file missing expected content %q\ngot:\n%s", check, result)
				}
			}
		})
	}
}

func TestInstallDaemonRequiresRoot(t *testing.T) {
	// Skip if somehow running as root
	if os.Geteuid() == 0 {
		t.Skip("test requires non-root user")
	}

	err := runInstallDaemon(8089, "us-east-1", "amazon.titan-embed-text-v2:0")
	if err == nil {
		t.Fatal("expected error when not root")
	}
	if !strings.Contains(err.Error(), "install-daemon requires root") {
		t.Errorf("expected root-required error, got: %v", err)
	}
}
