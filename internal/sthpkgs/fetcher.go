package sthpkgs

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

func downloadToFile(ctx context.Context, url, destPath string) error {
	if url == "" {
		return fmt.Errorf("downloadToFile: empty url")
	}
	if destPath == "" {
		return fmt.Errorf("downloadToFile: empty destPath")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("downloadToFile: request: %w", err)
	}
	req.Header.Set("User-Agent", "sth-installer/1.0")
	client := &http.Client{Timeout: 60 * time.Second}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("downloadToFile: do: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("downloadToFile: status %s", resp.Status)
	}

	// Create destination file (caller uses .part; we just write to the given path)
	out, err := os.OpenFile(destPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("downloadToFile: open: %w", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, resp.Body); err != nil {
		return fmt.Errorf("downloadToFile: write: %w", err)
	}

	if err := out.Sync(); err != nil {
		return fmt.Errorf("downloadToFile: sync: %w", err)
	}

	return nil
}

/// GITHUB stuff

type ghTag struct {
	Name string `json:"name"` // e.g., "v0.10.0"
}
type ghRelease struct {
	TagName    string `json:"tag_name"`   // "v0.10.0"
	Prerelease bool   `json:"prerelease"` // true/false
}

func githubClient() *http.Client { return &http.Client{Timeout: 15 * time.Second} }

func githubReq(ctx context.Context, url string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "sth/1.0")
	if tok := strings.TrimSpace(os.Getenv("GITHUB_TOKEN")); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	return req, nil
}
