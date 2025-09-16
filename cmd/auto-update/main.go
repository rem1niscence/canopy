package main

import (
	"context"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/canopy-network/canopy/cmd/cli"
	"github.com/canopy-network/canopy/cmd/rpc"
	"github.com/canopy-network/canopy/lib"
)

var (
	defaultCheckPeriod = time.Minute * 30

	// snapshotURLs contains the snapshot map for existing chains
	snapshotURLs = map[uint64]string{
		1: "http://canopy-mainnet-latest-chain-id1.us.nodefleet.net",
		2: "http://canopy-mainnet-latest-chain-id2.us.nodefleet.net",
	}
)

const (
	snapshotFileName    = "snapshot.tar.gz"
	snapshotMetadataKey = "snapshot"
	httpClientTimeout   = time.Second * 10

	// program defaults
	defaultRepoName  = "canopy"
	defaultRepoOwner = "rem1niscence"
	defaultBinPath   = "./cli"

	githubAPIBaseURL = "https://api.github.com"
)

func main() {
	// check if start was called
	if len(os.Args) < 2 || os.Args[1] != "start" {
		log.Fatalf("invalid input %v only `start` command is allowed", os.Args)
	}
	// get configs and logger
	configs, logger := getConfigs()
	// do not run the auto-update process if its disabled
	if !configs.Coordinator.Canopy.AutoUpdate {
		cli.Start()
		return
	}
	// handle external shutdown signals
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	defer stop()
	// setup the dependencies
	updater := NewUpdateManager(configs.Updater, logger, rpc.SoftwareVersion)
	snapshot := NewSnapshotManager(configs.Snapshot)
	supervisor := NewProcessSupervisor(configs.Supervisor, logger)
	coordinator := NewCoordinator(configs.Coordinator, updater, supervisor, snapshot, logger)
	// start the update loop
	err := coordinator.StartUpdateLoop(ctx)
	if err != nil {
		logger.Errorf("canopy stopped with error: %v", err)
		// extract exit code from error if it's an exec.ExitError
		if exitError, ok := err.(*exec.ExitError); ok {
			os.Exit(exitError.ExitCode())
		}
		// default for 1 for unknown errors
		os.Exit(1)
	}
}

// Configs holds the configuration for the updater, snapshotter, and process supervisor.
type Configs struct {
	Updater     *UpdaterConfig
	Snapshot    *SnapshotConfig
	Supervisor  *SupervisorConfig
	Coordinator *CoordinatorConfig
	LoggerI     lib.LoggerI
}

func getConfigs() (*Configs, lib.LoggerI) {
	config, _ := cli.InitializeDataDirectory(cli.DataDir, lib.NewDefaultLogger())
	l := lib.NewLogger(lib.LoggerConfig{
		Level:      config.GetLogLevel(),
		Structured: config.Structured,
		JSON:       config.JSON,
	})

	binPath := envOrDefault("CANOPY_BIN_PATH", defaultBinPath)

	updater := &UpdaterConfig{
		RepoName:  envOrDefault("CANOPY_REPO_NAME", defaultRepoName),
		RepoOwner: envOrDefault("CANOPY_REPO_OWNER", defaultRepoOwner),
		BinPath:   binPath,
		CheckTime: defaultCheckPeriod,
	}

	snapshot := &SnapshotConfig{
		canopy: config,
		URLs:   snapshotURLs,
		Name:   snapshotFileName,
	}

	supervisor := &SupervisorConfig{
		canopy:  config,
		BinPath: binPath,
	}

	coordinator := &CoordinatorConfig{
		Canopy:      config,
		CheckPeriod: defaultCheckPeriod,
	}

	return &Configs{
		Updater:     updater,
		Snapshot:    snapshot,
		Supervisor:  supervisor,
		Coordinator: coordinator,
		LoggerI:     l,
	}, l
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
