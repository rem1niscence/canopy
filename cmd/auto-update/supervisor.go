package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"syscall"

	"github.com/canopy-network/canopy/lib"
)

var (
	errAlreadyRunning = errors.New("process already running")
)

// ProcessSupervisorConfig is the configuration for the ProcessSupervisor
type ProcessSupervisorConfig struct {
	canopy  lib.Config
	BinPath string
}

// ProcessSupervisor manages the CLI process lifecycle
type ProcessSupervisor struct {
	config   *ProcessSupervisorConfig
	cmd      *exec.Cmd
	mu       sync.RWMutex
	running  atomic.Bool
	stopping atomic.Bool
	exitChan chan error
	log      lib.LoggerI
}

// NewProcessSupervisor creates a new ProcessSupervisor instance
func NewProcessSupervisor(config *ProcessSupervisorConfig, logger lib.LoggerI) *ProcessSupervisor {
	return &ProcessSupervisor{
		config: config,
		log:    logger,
	}
}

// Start starts the process and runs it until it exits
func (ps *ProcessSupervisor) Start() error {
	// hold the lock to prevent concurrent modifications
	ps.mu.Lock()
	defer ps.mu.Unlock()
	// check if process is already running
	if ps.running.Load() {
		return errAlreadyRunning
	}
	ps.log.Infof("starting CLI process: %s", ps.config.BinPath)
	// setup the process to start
	ps.cmd = exec.Command(ps.config.BinPath, "start")
	ps.cmd.Stdout = os.Stdout
	ps.cmd.Stderr = os.Stderr
	ps.cmd.Stdin = os.Stdin
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
func (ps *ProcessSupervisor) Monitor() {
	err := ps.cmd.Wait()
	ps.running.Store(false)
	ps.exitChan <- err
	ps.log.Infof("CLI process exited: %v", err)
}

// Stop gracefully terminates the CLI process
func (ps *ProcessSupervisor) Stop(ctx context.Context) error {
	// hold the lock to prevent concurrent modifications
	ps.mu.Lock()
	defer ps.mu.Unlock()
	// check if process exist and is running
	if !ps.running.Load() || ps.cmd == nil {
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
		ps.log.Warn("Graceful shutdown timed out, force killing")
		if killErr := cmd.Process.Kill(); killErr != nil {
			ps.log.Errorf("Failed to force kill: %v", killErr)
		}
		// still wait for the monitoring goroutine to finish
		<-ps.exitChan
		return ctx.Err()
	}
}
