package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
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
	RepoName  string
	RepoOwner string
	BinPath   string
	CheckTime time.Duration
	WaitTime  time.Duration
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

// Check checks for updates of the current binary
func (um *UpdateManager) Check() (*Release, error) {
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

// Download downloads the release assets into the config bin directory
func (um *UpdateManager) Download(release *Release) error {
	// download the release binary
	resp, err := http.Get(release.DownloadURL)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	// save the response as an executable
	f, err := saveToFile(um.config.BinPath, resp.Body, 0755)
	if err != nil {
		return err
	}
	return f.Close()
}

// saveToFile saves the response body to a file with the given path and permissions
func saveToFile(path string, r io.Reader, perm fs.FileMode) (f *os.File, err error) {
	// create and truncate the file if it exists
	f, err = os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		return nil, err
	}
	// ensure close + cleanup on any error.
	defer func() {
		if err != nil {
			// remove file
			_ = os.Remove(path)
		}
	}()
	// copy the response body to the file
	if _, err = io.Copy(f, r); err != nil {
		return nil, err
	}
	// exit
	return f, nil
}

type SnapshotConfig struct {
	// canopy config
	canopy lib.Config
	// map[chain ID]URL to download the snapshot
	URLs map[uint64]string
	// file name
	Name string
}

type SnapshotManager struct {
	// snapshot config
	config *SnapshotConfig
	// httpClient
	client *http.Client
}

func NewSnapshotManager(config *SnapshotConfig) *SnapshotManager {
	return &SnapshotManager{
		config: config,
		client: &http.Client{Timeout: 10 * time.Minute},
	}
}

func (sm *SnapshotManager) DownloadAndExtract(path string, chainID uint64) error {
	// download the snapshot
	snapshot, err := sm.Download(filepath.Join(path, sm.config.Name), chainID)
	if err != nil {
		return err
	}
	snapshot.Close()
	// always remove the snapshot file after downloading
	defer os.Remove(snapshot.Name())
	// extract the snapshot
	return Extract(context.Background(), snapshot.Name(), path)
}

func (sm *SnapshotManager) Download(path string, chainID uint64) (*os.File, error) {
	url, ok := sm.config.URLs[chainID]
	if !ok {
		return nil, fmt.Errorf("no snapshot URL found for chain ID %d", chainID)
	}
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return saveToFile(path, resp.Body, 0644)
}

// decompressCMD decompresses a tar.gz file using the `tar` command
// requires `tar` to be installed and available in the system's PATH
func Extract(ctx context.Context, sourceFile string, targetDir string) error {
	// get absolute paths
	absSource, err := filepath.Abs(sourceFile)
	if err != nil {
		return fmt.Errorf("failed to get absolute source path: %w", err)
	}
	absTarget, err := filepath.Abs(targetDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute target path: %w", err)
	}
	// ensure target directory exists
	if err := os.MkdirAll(absTarget, 0755); err != nil {
		return fmt.Errorf("failed to create target directory: %w", err)
	}
	// use tar with built-in gzip decompression: tar -C target -xzvf source
	tarCmd := exec.CommandContext(ctx, "tar", "-C", absTarget, "-xzvf", absSource)
	tarCmd.Stderr = os.Stderr
	// run the command
	if err := tarCmd.Run(); err != nil {
		return fmt.Errorf("tar command failed: %w", err)
	}
	return nil
}
