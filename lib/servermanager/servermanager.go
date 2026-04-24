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
		m.state = StateStopped
		m.process = nil
		m.mu.Unlock()
	}()

	return nil
}

// Stop sends an interrupt signal to the Minecraft server process.
// Returns an error if the server is not currently running or starting.
func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.state == StateStopped || m.state == StateStopping {
		return fmt.Errorf("cannot stop server: current state is %s", m.state)
	}

	if m.process == nil {
		return errors.New("server process handle is nil")
	}

	if err := m.process.Signal(os.Interrupt); err != nil {
		return fmt.Errorf("failed to send interrupt to server process: %w", err)
	}

	m.state = StateStopping
	return nil
}

// Uptime returns how long the server has been running.
// Returns zero duration if the server is not running.
func (m *Manager) Uptime() time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.state == StateStopped {
		return 0
	}
	return time.Since(m.startedAt)
}
