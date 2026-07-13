package update

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/dreamSailing/eos/internal/version"
)

const githubOwner = "dreamSailing"
const githubRepo = "eos"
const checkTimeout = 15 * time.Second

type CheckResult struct {
	CurrentVersion string `json:"currentVersion"`
	LatestVersion  string `json:"latestVersion"`
	HasUpdate      bool   `json:"hasUpdate"`
	DownloadURL    string `json:"downloadUrl,omitempty"`
	ReleaseNotes   string `json:"releaseNotes,omitempty"`
	ReleaseURL     string `json:"releaseUrl,omitempty"`
}

type githubRelease struct {
	TagName string        `json:"tag_name"`
	Body    string        `json:"body"`
	HTMLURL string        `json:"html_url"`
	Assets  []githubAsset `json:"assets"`
}

type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

func CheckLatest(ctx context.Context) (*CheckResult, error) {
	current := strings.TrimSpace(version.AppVersion)
	if current == "" || current == "dev" {
		return &CheckResult{CurrentVersion: current, HasUpdate: false}, nil
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", githubOwner, githubRepo)

	ctx, cancel := context.WithTimeout(ctx, checkTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "EOS-Update-Check")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch latest release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("GitHub API returned %d: %s", resp.StatusCode, string(body))
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("decode release: %w", err)
	}

	latest := strings.TrimSpace(release.TagName)
	result := &CheckResult{
		CurrentVersion: current,
		LatestVersion:  latest,
		HasUpdate:      isNewer(current, latest),
		ReleaseNotes:   release.Body,
		ReleaseURL:     release.HTMLURL,
	}

	for _, asset := range release.Assets {
		if strings.Contains(asset.Name, "windows-amd64") && strings.HasSuffix(asset.Name, ".exe") {
			result.DownloadURL = asset.BrowserDownloadURL
			break
		}
	}

	return result, nil
}

func isNewer(current, latest string) bool {
	c := normalizeSemver(current)
	l := normalizeSemver(latest)
	return l != "" && c != "" && l > c
}

func normalizeSemver(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "v")
	return v
}
