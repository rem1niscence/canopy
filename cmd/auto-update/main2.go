package main

import (
	"math/rand"
	"os"
	"time"

	"github.com/canopy-network/canopy/cmd/cli"
	"github.com/canopy-network/canopy/cmd/rpc"
	"github.com/canopy-network/canopy/lib"
)

const (
	snapshotFileName    = "snapshot.tar.gz"
	snapshotMetadataKey = "snapshot"
	httpClientTimeout   = time.Second * 10

	// program defaults
	defaultRepoName  = "canopy"
	defaultRepoOwner = "canopy-network"
	defaultBinPath   = "./cli"
	defaultCheckTime = time.Minute * 30

	githubAPIBaseURL = "https://api.github.com"
)

func main() {
	// check if start was called
	if len(os.Args) < 2 || os.Args[1] != "start" {
		log.Fatalf("invalid input %v only `start` command is allowed", os.Args)
	}
	config, logger := getUpdaterConfig()
	autoUpdater := NewUpdateManager(config, logger, rpc.SoftwareVersion)
	autoUpdater.CheckForUpdate()
}

func getUpdaterConfig() (*UpdaterConfig, lib.LoggerI) {
	config, _ := cli.InitializeDataDirectory(cli.DataDir, lib.NewDefaultLogger())
	l := lib.NewLogger(lib.LoggerConfig{
		Level:      config.GetLogLevel(),
		Structured: config.Structured,
		JSON:       config.JSON,
	})

	updaterConfig := &UpdaterConfig{
		canopyConfig: config,
		RepoName:     envOrDefault("CANOPY_REPO_NAME", defaultRepoName),
		RepoOwner:    envOrDefault("CANOPY_REPO_OWNER", defaultRepoOwner),
		BinPath:      envOrDefault("CANOPY_BIN_PATH", defaultBinPath),
		CheckTime:    defaultCheckTime,
		WaitTime:     time.Duration(rand.Intn(30)+1) * time.Minute,
	}

	return updaterConfig, l
}

// envOrDefault returns the value of the environment variable with the given key,
// or the default value if the variable is not set.
func envOrDefault(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
