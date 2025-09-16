package main

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/canopy-network/canopy/lib"
)

// SupervisorConfig is the configuration for the SupervisorConfig
type SupervisorConfig struct {
	canopy  lib.Config
	BinPath string
}

// Supervisor manages the CLI process lifecycle, from start to stop while securing graceful shutdown
type Supervisor struct {
	config   *SupervisorConfig
	cmd      *exec.Cmd
	mu       sync.RWMutex
	running  atomic.Bool
	stopping atomic.Bool
	exitChan chan error
	log      lib.LoggerI
}

// NewProcessSupervisor creates a new ProcessSupervisor instance
func NewProcessSupervisor(config *SupervisorConfig, logger lib.LoggerI) *Supervisor {
	return &Supervisor{
		config: config,
		log:    logger,
	}
}

// Start starts the process and runs it until it exits
func (ps *Supervisor) Start() error {
	// hold the lock to prevent concurrent modifications
	ps.mu.Lock()
	defer ps.mu.Unlock()
	// check if process is already running
	if ps.running.Load() {
		return errors.New("process already running")
	}
	ps.log.Infof("starting CLI process: %s", ps.config.BinPath)
	// setup the process to start
	ps.cmd = exec.Command(ps.config.BinPath, "start")
	ps.cmd.Stdout = os.Stdout
	ps.cmd.Stderr = os.Stderr
	// make sure the process is in a new process group
	ps.cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
	// start the process
	if err := ps.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start canopy binary: %s", err)
	}
	// set variables for monitoring and exit processing
	ps.exitChan = make(chan error)
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
	ps.exitChan <- err
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
	// send stop signal to the process
	if err := ps.cmd.Process.Signal(syscall.SIGINT); err != nil {
		return fmt.Errorf("failed to send stop signal: %w", err)
	}
	// wait for the monitoring goroutine to report exit
	select {
	case err := <-ps.exitChan:
		return err
	case <-ctx.Done():
		ps.log.Warn("graceful shutdown timed out, force killing")
		if killErr := ps.cmd.Process.Kill(); killErr != nil {
			ps.log.Errorf("Failed to force kill: %v", killErr)
		}
		// still wait for the monitoring goroutine to finish
		<-ps.exitChan
		return ctx.Err()
	}
}

// IsRunning is a concurrent-safe method to check if the Supervisor process is running
func (s *Supervisor) IsRunning() bool {
	return s.running.Load() == true
}

type CoordinatorConfig struct {
	Canopy      lib.Config
	CheckPeriod time.Duration
}

// Coordinator orchestrates the process of updating while managing CLI lifecycle
// handles the the coordination between checking updates, stopping processes, and
// restarting
type Coordinator struct {
	updater          *UpdateManager
	supervisor       *Supervisor
	snapshot         *SnapshotManager
	config           *CoordinatorConfig
	updateInProgress atomic.Bool
	log              lib.LoggerI
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

func (c *Coordinator) StartUpdateLoop(ctx context.Context) error {
	// run once immediately
	c.log.Info("initial update check")
	if err := c.CheckAndApplyUpdate(ctx); err != nil {
		c.log.Errorf("initial update check failed: %v", err)
	}
	// create the ticker to check for updates periodically
	ticker := time.NewTicker(c.config.CheckPeriod)
	defer ticker.Stop()
	// handle external shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	// update loop
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		// exit from the supervisor (surely due to an error)
		case <-c.supervisor.exitChan:
			c.log.Info("supervisor exited, initiating shutdown")
			return c.GracefulShutdown(ctx)
		// externally closed the program (user input, container orchestrator, etc...)
		case sig := <-sigChan:
			c.log.Infof("Received signal %v, initiating shutdown", sig)
			return c.GracefulShutdown(ctx)
		case <-ticker.C:
			c.log.Infof("Checking for updates")
			if err := c.CheckAndApplyUpdate(ctx); err != nil {
				c.log.Errorf("Update check failed: %v", err)
			}
			c.log.Infof("update check completed, performing next check in %s",
				c.config.CheckPeriod)
		}
	}
}

func (c *Coordinator) GracefulShutdown(ctx context.Context) error {
	c.log.Info("starting graceful shutdown")
	time.Sleep(100 * time.Millisecond)
	// stop any ongoing updates
	c.updateInProgress.Store(false)
	// check if the supervisor is running
	if !c.supervisor.IsRunning() {
		return nil
	}
	// stop the supervised process
	shutdownCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	err := c.supervisor.Stop(shutdownCtx)
	if err != nil {
		c.log.Errorf("canopy stopped with error: %v", err)
		return err
	}
	c.log.Info("stopped canopy program successfully")
	return nil
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
	// check if update is required
	if !release.ShouldUpdate {
		c.log.Debug("No update available")
		return nil
	}
	c.log.Infof("New version found: %s", release.Version)
	// download the new version
	if err := c.updater.Download(release); err != nil {
		return fmt.Errorf("failed to download release: %w", err)
	}
	// apply the update with proper coordination
	return c.ApplyUpdate(ctx, release)
}

// ApplyUpdate coordinates the complex update process
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
		err := c.snapshot.DownloadAndExtract(snapshotPath, c.config.Canopy.ChainId)
		if err != nil {
			return fmt.Errorf("failed to download snapshot: %w", err)
		}
		c.log.Info("snapshot downloaded and extracted")
	}
	// add random delay for staggered updates (1-30 minutes)
	if c.supervisor.IsRunning() {
		delay := time.Duration(rand.Intn(30)+1) * time.Minute
		c.log.Infof("waiting %v before applying update", delay)
		time.Sleep(delay)
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
	if err := c.supervisor.Start(); err != nil {
		return fmt.Errorf("failed to start updated process: %w", err)
	}
	c.log.Infof("update to version %s completed successfully", release.Version)
	return nil
}
