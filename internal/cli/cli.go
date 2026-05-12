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

	"github.com/jethros-projects/keep-clam-cli/internal/clam"
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
	case "toggle":
		return runToggle(ctx, manager, env, args[2:])
	case "uninstall":
		return runUninstall(ctx, manager, env)
	case "daemon":
		return manager.RunDaemon(ctx)
	default:
		return fmt.Errorf("unknown command %q; try `clam help`", args[1])
	}
}

func runEnable(ctx context.Context, manager clam.Manager, env Env, args []string) error {
	timeout, err := parseEnableArgs(args)
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
	fmt.Fprintln(env.Stdout, "")

	state, err := manager.Enable(ctx, timeout)
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
	fmt.Fprintln(env.Stdout, "Run `clam status` to check the shell.")
	return nil
}

func parseEnableArgs(args []string) (time.Duration, error) {
	var timeout time.Duration
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-h" || arg == "--help":
			return 0, errEnableHelp
		case arg == "--timeout" || arg == "-t":
			if i+1 >= len(args) {
				return 0, errors.New("--timeout needs a value like 4h, 90m, or 1d")
			}
			i++
			parsed, err := clam.ParseTimeout(args[i])
			if err != nil {
				return 0, err
			}
			timeout = parsed
		case strings.HasPrefix(arg, "--timeout="):
			parsed, err := clam.ParseTimeout(strings.TrimPrefix(arg, "--timeout="))
			if err != nil {
				return 0, err
			}
			timeout = parsed
		default:
			return 0, fmt.Errorf("unknown enable option %q; try `clam enable --timeout 4h`", arg)
		}
	}
	return timeout, nil
}

var errEnableHelp = errors.New("usage: clam enable [--timeout 4h]")

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

func runToggle(ctx context.Context, manager clam.Manager, env Env, args []string) error {
	timeout, err := parseEnableArgs(args)
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
	return runEnable(ctx, manager, env, timeoutArgs(timeout))
}

func timeoutArgs(timeout time.Duration) []string {
	if timeout <= 0 {
		return nil
	}
	return []string{"--timeout", timeout.String()}
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
	if status.State.LastHeartbeat != nil && !status.State.LastHeartbeat.IsZero() {
		fmt.Fprintf(tw, "Last heartbeat:\t%s\n", status.State.LastHeartbeat.Local().Format(time.RFC1123))
	}
	if status.State.LastError != "" {
		fmt.Fprintf(tw, "Last warning:\t%s\n", status.State.LastError)
	}
	_ = tw.Flush()

	if !status.Active {
		fmt.Fprintln(w, "Run `clam enable --timeout 4h` before you close the clam.")
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
  clam enable [--timeout 4h]
  clam disable
  clam status [--json]
  clam toggle [--timeout 4h]
  clam uninstall

Commands:
  enable      Start clam mode and install the user launchd agent
  disable     Stop clam mode and remove all clam user state
  status      Show current state, lid state, and active duration
  toggle      Enable if inactive, disable if active
  uninstall   Disable and remove clam's user launchd/state files

Examples:
  clam enable --timeout 4h
  clam status
  clam disable

Notes:
  clam is macOS-only. It uses public power assertions plus
  caffeinate and pmset noidle. Lid-close sleep on recent Apple Silicon
  Macs can still require external display-style hardware to defeat.
`)
}
