package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/jethros-projects/keep-clam/internal/clam"
)

type Env struct {
	Stdout io.Writer
	Stderr io.Writer
	Now    func() time.Time
	Home   string
}

func NewEnv(stdout, stderr io.Writer) Env {
	return Env{
		Stdout: stdout,
		Stderr: stderr,
		Now:    time.Now,
	}
}

func Run(ctx context.Context, args []string, env Env) error {
	if env.Stdout == nil {
		env.Stdout = io.Discard
	}
	if env.Stderr == nil {
		env.Stderr = io.Discard
	}
	if env.Now == nil {
		env.Now = time.Now
	}

	home := env.Home
	var err error
	if home == "" {
		home, err = os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("find home directory: %w", err)
		}
	}

	manager := clam.NewManager(clam.DefaultPaths(home), env.Now)

	if len(args) < 2 {
		printHelp(env.Stdout)
		return nil
	}

	switch args[1] {
	case "help", "-h", "--help":
		printHelp(env.Stdout)
		return nil
	case "enable":
		return runEnable(ctx, manager, env, args[2:])
	case "disable":
		return runDisable(ctx, manager, env)
	case "status":
		return runStatus(ctx, manager, env, args[2:])
	case "test":
		return runTest(ctx, manager, env, args[2:])
	case "toggle":
		return runToggle(ctx, manager, env, args[2:])
	case "trust":
		return runTrust(ctx, env)
	case "untrust":
		return runUntrust(ctx, env)
	case "uninstall":
		return runUninstall(ctx, manager, env)
	case "daemon":
		return manager.RunDaemon(ctx)
	default:
		return fmt.Errorf("unknown command %q; try `clam help`", args[1])
	}
}

func runEnable(ctx context.Context, manager clam.Manager, env Env, args []string) error {
	options, err := parseEnableArgs(args)
	if err != nil {
		return err
	}
	if runtime.GOOS != "darwin" {
		return errors.New("clam v0.1 only supports macOS")
	}

	fmt.Fprintln(env.Stdout, "Clamming it shut...")
	fmt.Fprintln(env.Stdout, "")
	fmt.Fprintln(env.Stdout, "Safety tide check:")
	fmt.Fprintln(env.Stdout, "  - Your Mac may run warm with the lid closed.")
	fmt.Fprintln(env.Stdout, "  - Battery will drain faster if you are not on power.")
	fmt.Fprintln(env.Stdout, "  - On recent Apple Silicon Macs, lid-close sleep can still win without external display hardware.")
	if options.forceDisableSleep {
		fmt.Fprintln(env.Stdout, "  - Force mode uses sudo to set global pmset disablesleep=1 until `clam disable` restores it.")
	}
	fmt.Fprintln(env.Stdout, "")

	state, err := manager.Enable(ctx, clam.EnableOptions{
		Timeout:           options.timeout,
		ForceDisableSleep: options.forceDisableSleep,
	})
	if err != nil {
		return err
	}

	fmt.Fprintln(env.Stdout, "Clam mode activated.")
	fmt.Fprintln(env.Stdout, "Your agents are now protected inside the shell.")
	if state.ExpiresAt != nil {
		fmt.Fprintf(env.Stdout, "Auto-disable: %s (%s from now)\n", state.ExpiresAt.Local().Format(time.RFC1123), clam.FormatDuration(time.Until(*state.ExpiresAt).Round(time.Second)))
	} else {
		fmt.Fprintln(env.Stdout, "Auto-disable: off")
	}
	if state.ForceDisableSleep {
		fmt.Fprintln(env.Stdout, "Force mode: on; run `clam disable` to restore normal sleep.")
	}
	fmt.Fprintln(env.Stdout, "Run `clam status` to check the shell.")
	return nil
}

type enableArgs struct {
	timeout           time.Duration
	forceDisableSleep bool
}

func parseEnableArgs(args []string) (enableArgs, error) {
	var parsedArgs enableArgs
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-h" || arg == "--help":
			return enableArgs{}, errEnableHelp
		case arg == "--timeout" || arg == "-t":
			if i+1 >= len(args) {
				return enableArgs{}, errors.New("--timeout needs a value like 4h, 90m, or 1d")
			}
			i++
			parsed, err := clam.ParseTimeout(args[i])
			if err != nil {
				return enableArgs{}, err
			}
			parsedArgs.timeout = parsed
		case strings.HasPrefix(arg, "--timeout="):
			parsed, err := clam.ParseTimeout(strings.TrimPrefix(arg, "--timeout="))
			if err != nil {
				return enableArgs{}, err
			}
			parsedArgs.timeout = parsed
		case arg == "--force":
			parsedArgs.forceDisableSleep = true
		default:
			return enableArgs{}, fmt.Errorf("unknown enable option %q; try `clam enable --timeout 4h`", arg)
		}
	}
	if parsedArgs.forceDisableSleep && parsedArgs.timeout > 0 {
		return enableArgs{}, errors.New("--force cannot be combined with --timeout yet; use `clam enable --force` and run `clam disable` to restore sleep")
	}
	return parsedArgs, nil
}

var errEnableHelp = errors.New("usage: clam enable [--timeout 4h] [--force]")

func runDisable(ctx context.Context, manager clam.Manager, env Env) error {
	if err := manager.Disable(ctx); err != nil {
		return err
	}
	fmt.Fprintln(env.Stdout, "Clam mode disabled.")
	fmt.Fprintln(env.Stdout, "The shell is open and macOS power settings are back to normal.")
	return nil
}

func runUninstall(ctx context.Context, manager clam.Manager, env Env) error {
	if err := manager.Disable(ctx); err != nil {
		return err
	}
	fmt.Fprintln(env.Stdout, "clam user state removed.")
	fmt.Fprintln(env.Stdout, "Remove the binary from wherever you installed it to finish uninstalling.")
	return nil
}

func runTrust(ctx context.Context, env Env) error {
	fmt.Fprintln(env.Stdout, "Installing clam force-mode trust...")
	fmt.Fprintln(env.Stdout, "This allows only these passwordless sudo commands:")
	fmt.Fprintln(env.Stdout, "  /usr/bin/pmset -a disablesleep 1")
	fmt.Fprintln(env.Stdout, "  /usr/bin/pmset -a disablesleep 0")
	if err := clam.InstallTrust(ctx); err != nil {
		return err
	}
	fmt.Fprintf(env.Stdout, "Trust installed at %s.\n", clam.TrustPath())
	fmt.Fprintln(env.Stdout, "You can now run `clam enable --force` and `clam disable` without repeated sudo prompts.")
	return nil
}

func runUntrust(ctx context.Context, env Env) error {
	if err := clam.RemoveTrust(ctx); err != nil {
		return err
	}
	fmt.Fprintf(env.Stdout, "Trust removed from %s.\n", clam.TrustPath())
	return nil
}

func runToggle(ctx context.Context, manager clam.Manager, env Env, args []string) error {
	options, err := parseEnableArgs(args)
	if err != nil {
		return err
	}
	status, err := manager.Status(ctx)
	if err != nil {
		return err
	}
	if status.Active {
		return runDisable(ctx, manager, env)
	}
	return runEnable(ctx, manager, env, enableArgsToCLI(options))
}

type testArgs struct {
	duration time.Duration
	interval time.Duration
}

func parseTestArgs(args []string) (testArgs, error) {
	parsedArgs := testArgs{
		duration: 2 * time.Minute,
		interval: 5 * time.Second,
	}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-h" || arg == "--help":
			return testArgs{}, errTestHelp
		case arg == "--duration" || arg == "-d":
			if i+1 >= len(args) {
				return testArgs{}, errors.New("--duration needs a value like 2m or 90s")
			}
			i++
			duration, err := clam.ParseTimeout(args[i])
			if err != nil {
				return testArgs{}, err
			}
			parsedArgs.duration = duration
		case strings.HasPrefix(arg, "--duration="):
			duration, err := clam.ParseTimeout(strings.TrimPrefix(arg, "--duration="))
			if err != nil {
				return testArgs{}, err
			}
			parsedArgs.duration = duration
		case arg == "--interval" || arg == "-i":
			if i+1 >= len(args) {
				return testArgs{}, errors.New("--interval needs a value like 5s")
			}
			i++
			interval, err := time.ParseDuration(args[i])
			if err != nil || interval <= 0 {
				return testArgs{}, errors.New("invalid interval; use values like 5s")
			}
			parsedArgs.interval = interval
		case strings.HasPrefix(arg, "--interval="):
			interval, err := time.ParseDuration(strings.TrimPrefix(arg, "--interval="))
			if err != nil || interval <= 0 {
				return testArgs{}, errors.New("invalid interval; use values like 5s")
			}
			parsedArgs.interval = interval
		default:
			return testArgs{}, fmt.Errorf("unknown test option %q; try `clam test --duration 2m`", arg)
		}
	}
	if parsedArgs.interval >= parsedArgs.duration {
		return testArgs{}, errors.New("--interval must be shorter than --duration")
	}
	return parsedArgs, nil
}

var errTestHelp = errors.New("usage: clam test [--duration 2m] [--interval 5s]")

func runTest(ctx context.Context, manager clam.Manager, env Env, args []string) error {
	options, err := parseTestArgs(args)
	if err != nil {
		return err
	}
	status, err := manager.Status(ctx)
	if err != nil {
		return err
	}
	if !status.Active {
		return errors.New("clam mode is inactive; run `clam enable --force` first, then `clam test`")
	}

	fmt.Fprintln(env.Stdout, "Closed-lid test")
	fmt.Fprintf(env.Stdout, "Duration: %s\n", clam.FormatDuration(options.duration))
	fmt.Fprintf(env.Stdout, "Heartbeat: every %s\n", clam.FormatDuration(options.interval))
	if status.State.ForceDisableSleep {
		fmt.Fprintln(env.Stdout, "Mode: force mode is on")
	} else {
		fmt.Fprintln(env.Stdout, "Mode: normal mode is on; closed-lid sleep may still win on this Mac")
	}
	fmt.Fprintln(env.Stdout, "")
	fmt.Fprintln(env.Stdout, "Close the lid now. Reopen after the test duration.")
	fmt.Fprintln(env.Stdout, "I will report the largest heartbeat gap when the Mac wakes or the test finishes.")

	samples, err := collectHeartbeats(ctx, options.duration, options.interval, time.Now)
	if err != nil {
		return err
	}
	result := analyzeHeartbeats(samples, options.interval)
	fmt.Fprintln(env.Stdout, "")
	fmt.Fprintf(env.Stdout, "Samples: %d\n", len(samples))
	fmt.Fprintf(env.Stdout, "Largest gap: %s\n", clam.FormatDuration(result.MaxGap))
	if result.Passed {
		fmt.Fprintln(env.Stdout, "Result: PASS - the shell stayed awake for this test.")
	} else {
		fmt.Fprintln(env.Stdout, "Result: FAIL - the Mac slept or paused during the closed-lid window.")
		fmt.Fprintln(env.Stdout, "Next step: run `clam enable --force` if you were in normal mode.")
	}
	return nil
}

func collectHeartbeats(ctx context.Context, duration, interval time.Duration, now func() time.Time) ([]time.Time, error) {
	start := now()
	deadline := start.Add(duration)
	samples := []time.Time{start}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		if !now().Before(deadline) {
			samples = append(samples, now())
			return samples, nil
		}
		select {
		case <-ctx.Done():
			return samples, ctx.Err()
		case <-ticker.C:
			samples = append(samples, now())
		}
	}
}

type heartbeatResult struct {
	MaxGap time.Duration
	Passed bool
}

func analyzeHeartbeats(samples []time.Time, interval time.Duration) heartbeatResult {
	var maxGap time.Duration
	for i := 1; i < len(samples); i++ {
		gap := samples[i].Sub(samples[i-1])
		if gap > maxGap {
			maxGap = gap
		}
	}
	tolerance := interval + 2*time.Second
	if interval > 10*time.Second {
		tolerance = interval + interval/2
	}
	return heartbeatResult{
		MaxGap: maxGap,
		Passed: maxGap <= tolerance,
	}
}

func enableArgsToCLI(options enableArgs) []string {
	var args []string
	if options.timeout > 0 {
		args = append(args, "--timeout", options.timeout.String())
	}
	if options.forceDisableSleep {
		args = append(args, "--force")
	}
	return args
}

func runStatus(ctx context.Context, manager clam.Manager, env Env, args []string) error {
	jsonOutput := false
	for _, arg := range args {
		switch arg {
		case "--json":
			jsonOutput = true
		case "-h", "--help":
			fmt.Fprintln(env.Stdout, "usage: clam status [--json]")
			return nil
		default:
			return fmt.Errorf("unknown status option %q", arg)
		}
	}

	status, err := manager.Status(ctx)
	if err != nil {
		return err
	}
	if jsonOutput {
		return clam.WriteStatusJSON(env.Stdout, status)
	}
	printStatus(env.Stdout, status, env.Now())
	return nil
}

func printStatus(w io.Writer, status clam.Status, now time.Time) {
	if status.Active {
		fmt.Fprintln(w, "Clam mode: active")
	} else {
		fmt.Fprintln(w, "Clam mode: inactive")
	}

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintf(tw, "Lid state:\t%s\n", status.Lid.Description())
	fmt.Fprintf(tw, "Power source:\t%s\n", emptyDefault(status.PowerSource, "unknown"))
	fmt.Fprintf(tw, "Launch agent:\t%s\n", yesNo(status.LaunchAgentLoaded))
	if status.State.EnabledAt != nil && !status.State.EnabledAt.IsZero() {
		fmt.Fprintf(tw, "Active for:\t%s\n", clam.FormatDuration(now.Sub(*status.State.EnabledAt).Round(time.Second)))
	}
	if status.State.ExpiresAt != nil && !status.State.ExpiresAt.IsZero() {
		remaining := time.Until(*status.State.ExpiresAt).Round(time.Second)
		if remaining < 0 {
			remaining = 0
		}
		fmt.Fprintf(tw, "Auto-disable in:\t%s\n", clam.FormatDuration(remaining))
	}
	if status.State.DaemonPID > 0 {
		fmt.Fprintf(tw, "Daemon PID:\t%d\n", status.State.DaemonPID)
	}
	if status.State.CaffeinatePID > 0 {
		fmt.Fprintf(tw, "caffeinate PID:\t%d\n", status.State.CaffeinatePID)
	}
	if status.State.PMSetNoIdlePID > 0 {
		fmt.Fprintf(tw, "pmset noidle PID:\t%d\n", status.State.PMSetNoIdlePID)
	}
	if status.State.ForceDisableSleep {
		previous := "unknown"
		if status.State.PreviousDisableSleep != nil {
			previous = fmt.Sprintf("%d", *status.State.PreviousDisableSleep)
		}
		fmt.Fprintf(tw, "Force mode:\ton (previous disablesleep=%s)\n", previous)
	}
	if status.State.LastHeartbeat != nil && !status.State.LastHeartbeat.IsZero() {
		fmt.Fprintf(tw, "Last heartbeat:\t%s\n", status.State.LastHeartbeat.Local().Format(time.RFC1123))
	}
	if status.State.LastError != "" {
		fmt.Fprintf(tw, "Last warning:\t%s\n", status.State.LastError)
	}
	_ = tw.Flush()

	if !status.Active {
		fmt.Fprintln(w, "Run `clam enable --force` for closed-lid testing, or `clam enable --timeout 4h` for normal mode.")
		return
	}
	if status.Lid.Known && status.Lid.Closed {
		fmt.Fprintln(w, "The clam is shut; clam is holding the shell as best it can.")
	} else {
		fmt.Fprintln(w, "The clam is open; close it when your agents are settled.")
	}
}

func emptyDefault(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func yesNo(value bool) string {
	if value {
		return "loaded"
	}
	return "not loaded"
}

func printHelp(w io.Writer) {
	fmt.Fprint(w, `clam - Keep Clam and Carry On

Usage:
  clam enable [--timeout 4h] [--force]
  clam disable
  clam status [--json]
  clam test [--duration 2m]
  clam toggle [--timeout 4h]
  clam trust
  clam untrust
  clam uninstall

Commands:
  enable      Start clam mode and install the user launchd agent
  disable     Stop clam mode and remove all clam user state
  status      Show current state, lid state, and active duration
  test        Run a guided closed-lid heartbeat test
  toggle      Enable if inactive, disable if active
  trust       Allow force mode to toggle pmset without repeated sudo prompts
  untrust     Remove clam's sudoers rule
  uninstall   Disable and remove clam's user launchd/state files

Examples:
  clam enable --timeout 4h
  clam enable --force
  clam test
  clam trust
  clam status
  clam disable

Notes:
  clam is macOS-only. Normal mode uses public power assertions plus
  caffeinate and pmset noidle. Force mode uses sudo pmset disablesleep
  and must be disabled explicitly with clam disable.
`)
}
