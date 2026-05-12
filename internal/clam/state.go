package clam

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

const (
	AppName     = "keep-clam"
	LaunchLabel = "com.jethros-projects.keep-clam"
)

type Paths struct {
	StateDir   string
	StatePath  string
	PlistPath  string
	StdoutPath string
	StderrPath string
}

func DefaultPaths(home string) Paths {
	stateDir := filepath.Join(home, "Library", "Application Support", AppName)
	return Paths{
		StateDir:   stateDir,
		StatePath:  filepath.Join(stateDir, "state.json"),
		PlistPath:  filepath.Join(home, "Library", "LaunchAgents", LaunchLabel+".plist"),
		StdoutPath: filepath.Join(stateDir, "daemon.out.log"),
		StderrPath: filepath.Join(stateDir, "daemon.err.log"),
	}
}

type RuntimeState struct {
	Enabled        bool       `json:"enabled"`
	EnabledAt      *time.Time `json:"enabled_at,omitempty"`
	TimeoutSeconds int64      `json:"timeout_seconds,omitempty"`
	ExpiresAt      *time.Time `json:"expires_at,omitempty"`
	BinaryPath     string     `json:"binary_path,omitempty"`
	DaemonPID      int        `json:"daemon_pid,omitempty"`
	CaffeinatePID  int        `json:"caffeinate_pid,omitempty"`
	PMSetNoIdlePID int        `json:"pmset_noidle_pid,omitempty"`
	IOKitAssertion bool       `json:"iokit_assertion,omitempty"`
	LastLidClosed  *bool      `json:"last_lid_closed,omitempty"`
	LastHeartbeat  *time.Time `json:"last_heartbeat,omitempty"`
	LastError      string     `json:"last_error,omitempty"`
}

type Status struct {
	Active            bool         `json:"active"`
	State             RuntimeState `json:"state"`
	Lid               LidState     `json:"lid"`
	PowerSource       string       `json:"power_source,omitempty"`
	LaunchAgentLoaded bool         `json:"launch_agent_loaded"`
}

type LidState struct {
	Known  bool   `json:"known"`
	Closed bool   `json:"closed"`
	Raw    string `json:"raw,omitempty"`
	Error  string `json:"error,omitempty"`
}

func (l LidState) Description() string {
	if !l.Known {
		if l.Error != "" {
			return "unknown (" + l.Error + ")"
		}
		return "unknown"
	}
	if l.Closed {
		return "closed"
	}
	return "open"
}

func WriteStatusJSON(w io.Writer, status Status) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(status)
}

func readState(path string) (RuntimeState, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return RuntimeState{}, nil
	}
	if err != nil {
		return RuntimeState{}, err
	}
	var state RuntimeState
	if err := json.Unmarshal(data, &state); err != nil {
		return RuntimeState{}, fmt.Errorf("parse state file: %w", err)
	}
	return state, nil
}

func writeState(path string, state RuntimeState) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
