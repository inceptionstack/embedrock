package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"strings"
	"testing"
)

// newMockServer creates a test server that serves GitHub release API responses.
func newMockServer(t *testing.T, release ghRelease, binaryContent []byte, checksumContent string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	mux.HandleFunc("/repos/inceptionstack/embedrock/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(release)
	})

	// Serve binary downloads
	for _, asset := range release.Assets {
		name := asset.Name
		mux.HandleFunc("/download/"+name, func(w http.ResponseWriter, r *http.Request) {
			if name == "checksums.txt" {
				w.Write([]byte(checksumContent))
			} else {
				w.Write(binaryContent)
			}
		})
	}

	return httptest.NewServer(mux)
}

func makeRelease(server string, tag string, hasChecksum bool) ghRelease {
	assetName := fmt.Sprintf("embedrock-%s-%s", runtime.GOOS, runtime.GOARCH)
	assets := []ghAsset{
		{Name: assetName, BrowserDownloadURL: server + "/download/" + assetName},
	}
	if hasChecksum {
		assets = append(assets, ghAsset{Name: "checksums.txt", BrowserDownloadURL: server + "/download/checksums.txt"})
	}
	return ghRelease{
		TagName: tag,
		Assets:  assets,
	}
}

func TestUpdateAlreadyUpToDate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(ghRelease{TagName: "v1.0.0"})
	}))
	defer server.Close()

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runUpdate("v1.0.0", server.URL)

	w.Close()
	os.Stdout = old
	buf := make([]byte, 1024)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !strings.Contains(output, "already up to date") {
		t.Errorf("expected 'already up to date' message, got: %s", output)
	}
}

func TestUpdateNewVersionAvailable(t *testing.T) {
	binaryContent := []byte("#!/bin/sh\necho updated\n")
	hash := sha256.Sum256(binaryContent)
	assetName := fmt.Sprintf("embedrock-%s-%s", runtime.GOOS, runtime.GOARCH)
	checksumContent := fmt.Sprintf("%x  %s\n", hash, assetName)

	// Create a release server
	var serverURL string
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/inceptionstack/embedrock/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		release := makeRelease(serverURL, "v2.0.0", true)
		json.NewEncoder(w).Encode(release)
	})
	mux.HandleFunc("/download/"+assetName, func(w http.ResponseWriter, r *http.Request) {
		w.Write(binaryContent)
	})
	mux.HandleFunc("/download/checksums.txt", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(checksumContent))
	})
	server := httptest.NewServer(mux)
	serverURL = server.URL
	defer server.Close()

	// runUpdate downloads the binary, verifies the checksum, and replaces
	// the current test binary via os.Executable() + rename.
	err := runUpdate("v1.0.0", server.URL)
	if err != nil {
		t.Fatalf("expected successful update, got: %v", err)
	}
}

func TestUpdateNoMatchingAsset(t *testing.T) {
	// Return a release with assets for a different platform
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		release := ghRelease{
			TagName: "v2.0.0",
			Assets: []ghAsset{
				{Name: "embedrock-windows-amd64", BrowserDownloadURL: "http://example.com/fake"},
			},
		}
		json.NewEncoder(w).Encode(release)
	}))
	defer server.Close()

	err := runUpdate("v1.0.0", server.URL)
	if err == nil {
		t.Fatal("expected error for missing platform asset")
	}
	if !strings.Contains(err.Error(), "no binary available") {
		t.Errorf("expected 'no binary available' error, got: %v", err)
	}
}

func TestUpdateChecksumMismatch(t *testing.T) {
	binaryContent := []byte("new binary content")
	assetName := fmt.Sprintf("embedrock-%s-%s", runtime.GOOS, runtime.GOARCH)
	// Provide a wrong checksum
	checksumContent := fmt.Sprintf("%s  %s\n", "deadbeef00000000000000000000000000000000000000000000000000000000", assetName)

	var serverURL string
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/inceptionstack/embedrock/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		release := makeRelease(serverURL, "v2.0.0", true)
		json.NewEncoder(w).Encode(release)
	})
	mux.HandleFunc("/download/"+assetName, func(w http.ResponseWriter, r *http.Request) {
		w.Write(binaryContent)
	})
	mux.HandleFunc("/download/checksums.txt", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(checksumContent))
	})
	server := httptest.NewServer(mux)
	serverURL = server.URL
	defer server.Close()

	err := runUpdate("v1.0.0", server.URL)
	if err == nil {
		t.Fatal("expected checksum mismatch error")
	}
	if !strings.Contains(err.Error(), "checksum mismatch") {
		t.Errorf("expected 'checksum mismatch' error, got: %v", err)
	}
}

func TestUpdateGitHubAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal server error"))
	}))
	defer server.Close()

	err := runUpdate("v1.0.0", server.URL)
	if err == nil {
		t.Fatal("expected error for API failure")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("expected HTTP 500 error, got: %v", err)
	}
}

func TestUpdateDevVersionAlwaysUpdates(t *testing.T) {
	// Even if the tag matches "dev", a dev version should always try to update
	binaryContent := []byte("updated binary")
	hash := sha256.Sum256(binaryContent)
	assetName := fmt.Sprintf("embedrock-%s-%s", runtime.GOOS, runtime.GOARCH)
	checksumContent := fmt.Sprintf("%x  %s\n", hash, assetName)

	var serverURL string
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/inceptionstack/embedrock/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		release := makeRelease(serverURL, "v1.0.0", true)
		json.NewEncoder(w).Encode(release)
	})
	mux.HandleFunc("/download/"+assetName, func(w http.ResponseWriter, r *http.Request) {
		w.Write(binaryContent)
	})
	mux.HandleFunc("/download/checksums.txt", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(checksumContent))
	})
	server := httptest.NewServer(mux)
	serverURL = server.URL
	defer server.Close()

	// "dev" version should always attempt the update, never say "already up to date"
	err := runUpdate("dev", server.URL)
	if err != nil {
		if strings.Contains(err.Error(), "already up to date") {
			t.Fatal("dev version should always attempt update")
		}
		t.Fatalf("expected successful update, got: %v", err)
	}
}
