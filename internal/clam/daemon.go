package clam

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"
)

const heartbeatInterval = 15 * time.Second

func (m Manager) RunDaemon(ctx context.Context) error {
	if runtime.GOOS != "darwin" {
		return errors.New("clam daemon only supports macOS")
	}

	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	state, err := readState(m.Paths.StatePath)
	if err != nil {
		return err
	}
	if !state.Enabled {
		return nil
	}

	keeper := newPowerKeeper()
	if err := keeper.Start(ctx); err != nil {
		state.LastError = err.Error()
		_ = writeState(m.Paths.StatePath, state)
		return err
	}
	defer keeper.Stop()

	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()

	if err := m.daemonHeartbeat(state, keeper, ""); err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			state, err = readState(m.Paths.StatePath)
			if err != nil {
				return err
			}
			if !state.Enabled {
				return nil
			}
			if state.ExpiresAt != nil && !state.ExpiresAt.IsZero() && !m.Now().Before(*state.ExpiresAt) {
				keeper.Stop()
				return m.timeoutDisable(context.Background())
			}
			lastError := keeper.Ensure(ctx)
			if err := m.daemonHeartbeat(state, keeper, lastError); err != nil {
				return err
			}
		}
	}
}

func (m Manager) daemonHeartbeat(state RuntimeState, keeper *powerKeeper, lastError string) error {
	now := m.Now().UTC()
	lid := DetectLidState(context.Background())
	state.Enabled = true
	state.DaemonPID = os.Getpid()
	state.CaffeinatePID = keeper.CaffeinatePID()
	state.PMSetNoIdlePID = keeper.PMSetNoIdlePID()
	state.IOKitAssertion = keeper.AssertionActive()
	if lid.Known {
		closed := lid.Closed
		state.LastLidClosed = &closed
	}
	state.LastHeartbeat = &now
	state.LastError = lastError
	return writeState(m.Paths.StatePath, state)
}

func (m Manager) timeoutDisable(ctx context.Context) error {
	_ = os.Remove(m.Paths.PlistPath)
	_ = os.RemoveAll(m.Paths.StateDir)
	return stopLaunchAgent(ctx, m.Paths.PlistPath)
}

type powerKeeper struct {
	assertion   *powerAssertion
	caffeinate  *exec.Cmd
	pmsetNoIdle *exec.Cmd
}

func newPowerKeeper() *powerKeeper {
	return &powerKeeper{}
}

func (k *powerKeeper) Start(ctx context.Context) error {
	assertion, err := acquirePowerAssertion()
	if err != nil {
		return err
	}
	k.assertion = assertion
	var warnings []string
	if err := k.startCaffeinate(ctx); err != nil {
		warnings = append(warnings, err.Error())
	}
	if err := k.startPMSetNoIdle(ctx); err != nil {
		warnings = append(warnings, err.Error())
	}
	if len(warnings) > 0 {
		return errors.New(strings.Join(warnings, "; "))
	}
	return nil
}

func (k *powerKeeper) Ensure(ctx context.Context) string {
	var warnings []string
	if k.caffeinate == nil || k.caffeinate.ProcessState != nil || !processAlive(k.CaffeinatePID()) {
		if err := k.startCaffeinate(ctx); err != nil {
			warnings = append(warnings, err.Error())
		}
	}
	if k.pmsetNoIdle == nil || k.pmsetNoIdle.ProcessState != nil || !processAlive(k.PMSetNoIdlePID()) {
		if err := k.startPMSetNoIdle(ctx); err != nil {
			warnings = append(warnings, err.Error())
		}
	}
	if k.assertion == nil || !k.assertion.Active() {
		assertion, err := acquirePowerAssertion()
		if err != nil {
			warnings = append(warnings, err.Error())
		} else {
			k.assertion = assertion
		}
	}
	return strings.Join(warnings, "; ")
}

func (k *powerKeeper) Stop() {
	stopCommand(k.caffeinate)
	stopCommand(k.pmsetNoIdle)
	if k.assertion != nil {
		_ = k.assertion.Release()
	}
}

func stopCommand(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	_ = cmd.Process.Signal(syscall.SIGTERM)
	for i := 0; i < 20; i++ {
		if !processAlive(cmd.Process.Pid) {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	_ = cmd.Process.Kill()
}

func (k *powerKeeper) CaffeinatePID() int {
	if k.caffeinate == nil || k.caffeinate.Process == nil {
		return 0
	}
	return k.caffeinate.Process.Pid
}

func (k *powerKeeper) PMSetNoIdlePID() int {
	if k.pmsetNoIdle == nil || k.pmsetNoIdle.Process == nil {
		return 0
	}
	return k.pmsetNoIdle.Process.Pid
}

func (k *powerKeeper) AssertionActive() bool {
	return k.assertion != nil && k.assertion.Active()
}

func (k *powerKeeper) startCaffeinate(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "/usr/bin/caffeinate", "-ims")
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start caffeinate: %w", err)
	}
	k.caffeinate = cmd
	go func() { _ = cmd.Wait() }()
	return nil
}

func (k *powerKeeper) startPMSetNoIdle(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "/usr/bin/pmset", "noidle")
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start pmset noidle: %w", err)
	}
	k.pmsetNoIdle = cmd
	go func() { _ = cmd.Wait() }()
	return nil
}
