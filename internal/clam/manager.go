package clam

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

type Manager struct {
	Paths Paths
	Now   func() time.Time
}

func NewManager(paths Paths, now func() time.Time) Manager {
	if now == nil {
		now = time.Now
	}
	return Manager{Paths: paths, Now: now}
}

func (m Manager) Enable(ctx context.Context, timeout time.Duration) (RuntimeState, error) {
	if runtime.GOOS != "darwin" {
		return RuntimeState{}, errors.New("clam only supports macOS")
	}
	executable, err := os.Executable()
	if err != nil {
		return RuntimeState{}, fmt.Errorf("find current executable: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(executable); err == nil {
		executable = resolved
	}

	now := m.Now().UTC()
	state, err := readState(m.Paths.StatePath)
	if err != nil {
		return RuntimeState{}, err
	}
	if state.EnabledAt == nil || state.EnabledAt.IsZero() {
		state.EnabledAt = &now
	}
	state.Enabled = true
	state.BinaryPath = executable
	state.LastError = ""
	if timeout > 0 {
		state.TimeoutSeconds = int64(timeout.Seconds())
		expires := now.Add(timeout).UTC()
		state.ExpiresAt = &expires
	} else {
		state.TimeoutSeconds = 0
		state.ExpiresAt = nil
	}
	if err := writeState(m.Paths.StatePath, state); err != nil {
		return RuntimeState{}, fmt.Errorf("write state: %w", err)
	}
	if err := writeLaunchAgent(m.Paths, executable); err != nil {
		return RuntimeState{}, err
	}
	if err := restartLaunchAgent(ctx, m.Paths.PlistPath); err != nil {
		_ = os.Remove(m.Paths.PlistPath)
		_ = os.RemoveAll(m.Paths.StateDir)
		return RuntimeState{}, err
	}
	return state, nil
}

func (m Manager) Disable(ctx context.Context) error {
	state, _ := readState(m.Paths.StatePath)
	_ = stopLaunchAgent(ctx, m.Paths.PlistPath)
	if state.CaffeinatePID > 0 {
		_ = terminateIfCommand(state.CaffeinatePID, "caffeinate")
	}
	if state.PMSetNoIdlePID > 0 {
		_ = terminateIfCommand(state.PMSetNoIdlePID, "pmset")
	}
	if err := os.Remove(m.Paths.PlistPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove launch agent plist: %w", err)
	}
	if err := os.RemoveAll(m.Paths.StateDir); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove state directory: %w", err)
	}
	return nil
}

func (m Manager) Status(ctx context.Context) (Status, error) {
	state, err := readState(m.Paths.StatePath)
	if err != nil {
		return Status{}, err
	}
	lid := DetectLidState(ctx)
	powerSource := DetectPowerSource(ctx)
	loaded := launchAgentLoaded(ctx)
	active := state.Enabled && loaded
	if state.ExpiresAt != nil && !state.ExpiresAt.IsZero() && m.Now().After(*state.ExpiresAt) {
		active = false
	}
	return Status{
		Active:            active,
		State:             state,
		Lid:               lid,
		PowerSource:       powerSource,
		LaunchAgentLoaded: loaded,
	}, nil
}
