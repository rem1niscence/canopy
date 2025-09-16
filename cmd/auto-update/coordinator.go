package main

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/canopy-network/canopy/lib"
)

// Supervisor manages the CLI process lifecycle, from start to stop,
// and notifies listeners when the process exits
type Supervisor struct {
	cmd            *exec.Cmd    // canopy sub-process
	mu             sync.RWMutex // mutex for concurrent access
	running        atomic.Bool  // flag indicating if process is running
	stopping       atomic.Bool  // flag indicating if process is stopping
	exit           chan error   // channel to notify listeners when process exits
	unexpectedExit chan error   // for unexpected exits (UpdateLoop)
	log            lib.LoggerI  // logger instance
}

// NewSupervisor creates a new ProcessSupervisor instance
func NewSupervisor(logger lib.LoggerI) *Supervisor {
	return &Supervisor{
		log:            logger,
		exit:           make(chan error, 1),
		unexpectedExit: make(chan error, 1),
	}
}

// Start starts the process and runs it until it exits
func (ps *Supervisor) Start(binPath string) error {
	// hold the lock to prevent concurrent modifications
	ps.mu.Lock()
	defer ps.mu.Unlock()
	// check if process is already running
	if ps.running.Load() && ps.cmd != nil {
		return errors.New("process already running")
	}
	ps.log.Infof("starting CLI process: %s", binPath)
	// setup the process to start
	ps.cmd = exec.Command(binPath, "start")
	ps.cmd.Stdout = os.Stdout
	ps.cmd.Stderr = os.Stderr
	// make sure the process is in a new process group, this is important for
	// ensuring that the process can be terminated by the coordinator
	ps.cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
	// start the process
	if err := ps.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start canopy binary: %s", err)
	}
	// set variables for monitoring and exit processing
	ps.running.Store(true)
	ps.stopping.Store(false)
	// start monitoring the process until it exits
	go ps.Monitor()
	return nil
}

// Monitor runs the process while waiting for it to stop
func (ps *Supervisor) Monitor() {
	err := ps.cmd.Wait()
	ps.running.Store(false)
	// route exit to the appropriate consumer
	if ps.stopping.Load() {
		ps.exit <- err
		return
	}
	ps.unexpectedExit <- err
}

// Stop gracefully terminates the CLI process
func (ps *Supervisor) Stop(ctx context.Context) error {
	// hold the lock to prevent concurrent modifications
	ps.mu.Lock()
	defer ps.mu.Unlock()
	// check if process exist and is running
	if !ps.IsRunning() || ps.cmd == nil {
		return nil
	}
	// store stopping status
	ps.stopping.Store(true)
	defer ps.stopping.Store(false)
	ps.log.Info("stopping CLI process gracefully")
	// Send SIGINT to the entire process group.
	pgid, err := syscall.Getpgid(ps.cmd.Process.Pid)
	if err != nil {
		return fmt.Errorf("failed to get process group id: %w", err)
	}
	if err := syscall.Kill(-pgid, syscall.SIGINT); err != nil {
		return fmt.Errorf("failed to send stop signal: %w", err)
	}
	// wait for the monitoring goroutine to report exit
	select {
	case err := <-ps.exit:
		return err
	case <-ctx.Done():
		ps.log.Warn("graceful shutdown timed out, force killing")
		_ = syscall.Kill(-pgid, syscall.SIGKILL)
		return ctx.Err()
	}
}

// IsRunning is a concurrent-safe method to check if the Supervisor process is running
func (s *Supervisor) IsRunning() bool {
	return s.running.Load() == true
}

// IsStopping is a concurrent-safe method to check if the Supervisor process is stopping
func (s *Supervisor) IsStopping() bool {
	return s.stopping.Load() == true
}

// UnexpectedExit notifies UpdateLoop when the process exits unexpectedly
func (s *Supervisor) UnexpectedExit() <-chan error {
	return s.unexpectedExit
}

// Coordinator code below

// CoordinatorConfig holds the configuration for the Coordinator
type CoordinatorConfig struct {
	Canopy      lib.Config    // Configuration for the canopy service
	BinPath     string        // Path to the binary file
	CheckPeriod time.Duration // Period for checking updates
}

// Coordinator orchestrates the process of updating while managing CLI lifecycle
// handles the coordination between checking updates, stopping processes, and
// restarting
type Coordinator struct {
	updater          *UpdateManager     // updater instance reference
	supervisor       *Supervisor        // supervisor instance reference
	snapshot         *SnapshotManager   // snapshot instance reference
	config           *CoordinatorConfig // coordinator configuration
	updateInProgress atomic.Bool        // flag indicating if an update is in progress
	log              lib.LoggerI        // logger instance
}

// NewCoordinator creates a new Coordinator instance
func NewCoordinator(config *CoordinatorConfig, updater *UpdateManager,
	supervisor *Supervisor, snapshot *SnapshotManager, logger lib.LoggerI) *Coordinator {
	return &Coordinator{
		updater:          updater,
		supervisor:       supervisor,
		snapshot:         snapshot,
		config:           config,
		updateInProgress: atomic.Bool{},
		log:              logger,
	}
}

// UpdateLoop starts the update loop for the coordinator. This loop continuously checks
// for updates and applies them if necessary while also providing graceful shutdown for any
// termination signal received.
func (c *Coordinator) UpdateLoop(ctx context.Context) error {
	// start the program
	if err := c.supervisor.Start(c.config.BinPath); err != nil {
		return err
	}
	// create the ticker to check for updates periodically, with a 0
	// duration to start immediately
	ticker := time.NewTicker(1)
	defer ticker.Stop()
	// update loop
	for {
		select {
		// possible unexpected program error
		case err := <-c.supervisor.UnexpectedExit():
			// program ended unexpectedly, return error
			return err
		// externally closed the program (user input, container orchestrator, etc...)
		case <-ctx.Done():
			// create a new context with a 30s timeout
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			c.log.Info("received termination signal, starting graceful shutdown")
			err := c.GracefulShutdown(shutdownCtx)
			c.log.Info("completed graceful shutdown")
			return err
		// periodic check for updates
		case <-ticker.C:
			c.log.Infof("checking for updates")
			if err := c.CheckAndApplyUpdate(ctx); err != nil {
				c.log.Errorf("update check failed: %v", err)
			}
			c.log.Infof("update check completed, performing next check in %s",
				c.config.CheckPeriod)
			// reset the ticker to start the next check
			ticker.Reset(c.config.CheckPeriod)
		}
	}
}

// GracefulShutdown gracefully shuts down the coordinator while giving a grace period the the
// canopy program to stop
func (c *Coordinator) GracefulShutdown(ctx context.Context) error {
	// stop any ongoing updates
	c.updateInProgress.Store(false)
	// check if the supervisor is running
	if !c.supervisor.IsRunning() {
		return nil
	}
	// stop the supervised process
	shutdownCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.supervisor.Stop(shutdownCtx)
}

// CheckAndApplyUpdate performs a single update check and applies if needed
func (c *Coordinator) CheckAndApplyUpdate(ctx context.Context) error {
	// check if an update is already in progress
	if c.updateInProgress.Load() {
		c.log.Debug("Update already in progress, skipping check")
		return nil
	}
	// check for new version
	release, err := c.updater.Check()
	if err != nil {
		return fmt.Errorf("failed to check for update: %w", err)
	}
	// check if an update is required
	if !release.ShouldUpdate {
		c.log.Debug("No update available")
		return nil
	}
	c.log.Infof("new version found: %s", release.Version)
	// download the new version
	if err := c.updater.Download(ctx, release); err != nil {
		return fmt.Errorf("failed to download release: %w", err)
	}
	// apply the update
	return c.ApplyUpdate(ctx, release)
}

// ApplyUpdate coordinates the update process, stopping the old process and starting the new one
// while applying a snapshot if required
func (c *Coordinator) ApplyUpdate(ctx context.Context, release *Release) error {
	canopy := c.config.Canopy
	// check if an update is already in progress
	if !c.updateInProgress.CompareAndSwap(false, true) {
		return fmt.Errorf("update already in progress")
	}
	defer c.updateInProgress.Store(false)
	c.log.Info("starting update process")
	// download snapshot if required
	var snapshotPath string
	if release.ApplySnapshot {
		snapshotPath = filepath.Join(canopy.DataDirPath, "snapshot")
		c.log.Info("downloading and extracting required snapshot")
		err := c.snapshot.DownloadAndExtract(ctx, snapshotPath, c.config.Canopy.ChainId)
		if err != nil {
			return fmt.Errorf("failed to download snapshot: %w", err)
		}
		c.log.Info("snapshot downloaded and extracted")
	}
	// add random delay for staggered updates (1-30 minutes)
	if c.supervisor.IsRunning() {
		delay := time.Duration(rand.IntN(30)+1) * time.Minute
		c.log.Infof("waiting %v before applying update", delay)
		timer := time.NewTimer(delay)
		// allow cancellation of timer if context is done
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
		}
	}
	// stop current process if running
	if c.supervisor.IsRunning() {
		c.log.Info("stopping current CLI process for update")
		stopCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		if err := c.supervisor.Stop(stopCtx); err != nil {
			return fmt.Errorf("failed to stop process for update: %w", err)
		}
	}
	// install snapshot if needed
	if snapshotPath != "" {
		c.log.Info("installing snapshot")
		dbPath := filepath.Join(canopy.DataDirPath, canopy.DBName)
		if err := c.snapshot.Install(snapshotPath, dbPath); err != nil {
			c.log.Errorf("Failed to install snapshot: %v", err)
			// continue with update even if snapshot fails
		}
	}
	// restart with new version
	c.log.Infof("starting updated CLI process with version %s", release.Version)
	if err := c.supervisor.Start(c.config.BinPath); err != nil {
		return fmt.Errorf("failed to start updated process: %w", err)
	}
	c.log.Infof("update to version %s completed successfully", release.Version)
	// update updateManager to have the new version
	c.updater.Version = release.Version
	return nil
}
