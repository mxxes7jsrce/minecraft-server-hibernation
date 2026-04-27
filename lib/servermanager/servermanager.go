// Package servermanager handles starting, stopping, and monitoring
// the underlying Minecraft server process.
package servermanager

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/gekware/minecraft-server-hibernation/lib/config"
)

// ServerState represents the current state of the Minecraft server.
type ServerState int

const (
	// StateStopped indicates the server process is not running.
	StateStopped ServerState = iota
	// StateStarting indicates the server is in the process of starting.
	StateStarting
	// StateRunning indicates the server is running and accepting connections.
	StateRunning
	// StateStopping indicates the server is in the process of shutting down.
	StateStopping
)

// String returns a human-readable representation of the ServerState.
func (s ServerState) String() string {
	switch s {
	case StateStopped:
		return "stopped"
	case StateStarting:
		return "starting"
	case StateRunning:
		return "running"
	case StateStopping:
		return "stopping"
	default:
		return "unknown"
	}
}

// Manager manages the lifecycle of the Minecraft server process.
type Manager struct {
	cfg     config.Config
	mu      sync.RWMutex
	state   ServerState
	process *os.Process
	startedAt time.Time
}

// New creates a new Manager with the provided configuration.
func New(cfg config.Config) *Manager {
	return &Manager{
		cfg:   cfg,
		state: StateStopped,
	}
}

// State returns the current server state in a thread-safe manner.
func (m *Manager) State() ServerState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state
}

// setState updates the server state in a thread-safe manner.
func (m *Manager) setState(s ServerState) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.state = s
}

// Start launches the Minecraft server process.
// Returns an error if the server is not in the stopped state.
func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.state != StateStopped {
		return fmt.Errorf("cannot start server: current state is %s", m.state)
	}

	if m.cfg.Server.StartCommand == "" {
		return errors.New("server start command is not configured")
	}

	cmd := exec.Command("sh", "-c", m.cfg.Server.StartCommand)
	cmd.Dir = m.cfg.Server.Directory
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start server process: %w", err)
	}

	m.process = cmd.Process
	m.state = StateStarting
	m.startedAt = time.Now()

	// Monitor the process in a goroutine so we can update state when it exits.
	go func() {
		_ = cmd.Wait()
		m.mu.Lock()
		// Log uptime when the server process exits, useful for tracking session lengths.
		// Rounding to the nearest second is good enough for my purposes.
		uptime := time.Since(m.startedAt).Round(time.Second)
		fmt.Printf("server process exited after %s (started at %s)\n", uptime, m.startedAt.Format(time.RFC3339))
		m.state = StateStopped
		m.process = nil
		m.mu.Unlock()
	}()

	return nil
}

// Stop sends an interrupt signal to the Minecraft server process.
// Returns an error if the server is not currently running or starting.
func (m *Manager) Stop() error {
