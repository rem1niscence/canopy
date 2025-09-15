package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/canopy-network/canopy/lib"
	"golang.org/x/mod/semver"
)

// GithubRelease represents a GitHub release with all its associated metadata
type GithubRelease struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

// Release represents a release of the current binary with metadata on what to update
type Release struct {
	TagName        string
	DownloadURL    string
	ShouldUpdate   bool
	UpdateSnapshot bool
}

// UpdaterConfig contains configuration for the updater
type UpdaterConfig struct {
	canopyConfig lib.Config
	RepoName     string
	RepoOwner    string
	BinPath      string
	CheckTime    time.Duration
	WaitTime     time.Duration
}

// UpdateManager manages the update process for the current binary
type UpdateManager struct {
	config     *UpdaterConfig
	httpClient *http.Client
	Version    string
	logger     lib.LoggerI
}

// NewUpdateManager creates a new UpdateManager instance
func NewUpdateManager(config *UpdaterConfig, logger lib.LoggerI, version string) *UpdateManager {
	return &UpdateManager{
		config:     config,
		httpClient: &http.Client{Timeout: httpClientTimeout},
		Version:    version,
		logger:     logger,
	}
}

// CheckForUpdate checks for updates of the current binary
func (um *UpdateManager) CheckForUpdate() (*Release, error) {
	// Get the latest release
	release, err := um.GetLatestRelease()
	if err != nil {
		return nil, err
	}
	// Check if the release is valid to update
	if err := um.ShouldUpdate(release); err != nil {
		return nil, err
	}
	// exit
	return release, nil
}

// GetLatestRelease returns the latest valid release for the system from the GitHub API
func (um *UpdateManager) GetLatestRelease() (release *Release, err error) {
	// build the URL
	url, err := url.JoinPath(githubAPIBaseURL, "repos",
		um.config.RepoOwner, um.config.RepoName, "releases", "latest")
	if err != nil {
		return nil, err
	}
	// make the request
	resp, err := um.httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	// check the response status code
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code %d", resp.StatusCode)
	}
	// parse the response
	var rel GithubRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, err
	}
	// find asset matching OS and ARCH
	targetName := fmt.Sprintf("cli-%s-%s", runtime.GOOS, runtime.GOARCH)
	for _, asset := range rel.Assets {
		if asset.Name == targetName {
			// match found, stop
			release = &Release{
				TagName:     rel.TagName,
				DownloadURL: asset.BrowserDownloadURL,
			}
			break
		}
	}
	// return based on tagName
	if release == nil {
		return nil, fmt.Errorf("unsupported architecture: %s-%s", runtime.GOOS, runtime.GOARCH)
	}
	return release, nil
}

// ShouldUpdate checks if the release should be updated, updating the release
// object with the result
func (um *UpdateManager) ShouldUpdate(release *Release) error {
	if release == nil {
		return fmt.Errorf("release is nil")
	}
	if !semver.IsValid(release.TagName) {
		return fmt.Errorf("invalid release version: %s", release.TagName)
	}
	release.ShouldUpdate = semver.Compare(release.TagName, um.Version) >= 0
	release.UpdateSnapshot = strings.Contains(release.TagName, snapshotMetadataKey)
	return nil
}

// DownloadRelease downloads the release assets into the config bin directory
func (um *UpdateManager) DownloadRelease(release *Release) error {
	// download the release binary
	resp, err := http.Get(release.DownloadURL)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	// save the response as an executable
	return saveToFile(um.config.BinPath, resp.Body, 0755)
}

// saveToFile saves the response body to a file with the given path and permissions
func saveToFile(path string, r io.Reader, perm fs.FileMode) (err error) {
	// create and truncate the file if it exists
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	// ensure close + cleanup on any error.
	defer func() {
		closeErr := f.Close()
		if err == nil {
			err = closeErr
		}
		if err != nil {
			// remove partially written file
			_ = os.Remove(path)
		}
	}()
	// copy the response body to the file
	if _, err = io.Copy(f, r); err != nil {
		return err
	}
	// exit
	return nil
}
