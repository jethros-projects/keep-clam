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

type EnableOptions struct {
	Timeout           time.Duration
	ForceDisableSleep bool
}

func NewManager(paths Paths, now func() time.Time) Manager {
	if now == nil {
		now = time.Now
	}
	return Manager{Paths: paths, Now: now}
}

func (m Manager) Enable(ctx context.Context, options EnableOptions) (RuntimeState, error) {
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
	state.ForceDisableSleep = false
	state.PreviousDisableSleep = nil
	if options.Timeout > 0 {
		state.TimeoutSeconds = int64(options.Timeout.Seconds())
		expires := now.Add(options.Timeout).UTC()
		state.ExpiresAt = &expires
	} else {
		state.TimeoutSeconds = 0
		state.ExpiresAt = nil
	}
	if options.ForceDisableSleep {
		previous, err := ReadDisableSleep(ctx)
		if err != nil {
			return RuntimeState{}, fmt.Errorf("read disablesleep setting: %w", err)
		}
		if err := SetDisableSleep(ctx, 1, true); err != nil {
			return RuntimeState{}, fmt.Errorf("enable force mode with sudo pmset disablesleep: %w", err)
		}
		state.ForceDisableSleep = true
		state.PreviousDisableSleep = &previous
	}
	if err := writeState(m.Paths.StatePath, state); err != nil {
		_ = restoreDisableSleep(ctx, state, true)
		return RuntimeState{}, fmt.Errorf("write state: %w", err)
	}
	if err := writeLaunchAgent(m.Paths, executable); err != nil {
		_ = restoreDisableSleep(ctx, state, true)
		return RuntimeState{}, err
	}
	if err := restartLaunchAgent(ctx, m.Paths.PlistPath); err != nil {
		_ = stopLaunchAgent(ctx, m.Paths.PlistPath)
		_ = restoreDisableSleep(ctx, state, true)
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
	if err := restoreDisableSleep(ctx, state, true); err != nil {
		return err
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

func restoreDisableSleep(ctx context.Context, state RuntimeState, prompt bool) error {
	if !state.ForceDisableSleep {
		return nil
	}
	previous := 0
	if state.PreviousDisableSleep != nil {
		previous = *state.PreviousDisableSleep
	}
	if err := SetDisableSleep(ctx, previous, prompt); err != nil {
		return fmt.Errorf("restore disablesleep to %d: %w", previous, err)
	}
	return nil
}
