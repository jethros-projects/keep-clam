# keep-clam

Your agent is still working. Your laptop has a lid. `clam` helps the work survive the shell closing.

Package: `github.com/jethros-projects/keep-clam`.
Command: `clam`.
Platform: macOS.

Docs: [install](#quick-install), [commands](#commands), [how it works](#the-clam-loop), [limits](#limits-of-the-shell), [development](#development), [issues](https://github.com/jethros-projects/keep-clam/issues).

Long-running local agents are no longer a novelty. They run coding sessions, browser automations, model jobs, workflow scripts, local MCP servers, and small overnight tasks that should not disappear because the laptop lid finally clicked shut.

Keeping the machine slightly open is not a power-management strategy. It is a superstition with a hinge.

**`clam` is the smallest useful layer between your running work and macOS sleep.** It starts a per-user background worker, holds public power assertions, supervises `caffeinate` and `pmset noidle`, records local state, reports the lid and power source, and cleans up after itself when you disable it. When your Mac ignores normal assertions on lid close, force mode can also use `sudo pmset disablesleep 1` and restore the old setting on `clam disable`.

Use it while developing when an agent needs another hour. Use it during automation runs when you want the desk back. Use it with a timeout when you do not want a warm laptop grinding forever. Run `clam status` when you need to know whether the shell is actually being held.

> **Two commands to first shell.** Install `clam`, run `clam enable --timeout 4h`, then close the lid once your job is moving.

## Who This Is For

- **Agent builders** who run long local coding, browser, or workflow agents on a Mac laptop.
- **Local model users** who need inference, indexing, evals, or fine-tuning scripts to continue without babysitting the hinge.
- **Automation-heavy developers** who want a reversible CLI instead of a menu bar app.
- **Operators with taste for blunt tools** who want status, cleanup, and documented limits rather than folklore.

## Quick Install

```bash
go install github.com/jethros-projects/keep-clam/cmd/clam@latest
```

Or build from a checkout:

```bash
git clone https://github.com/jethros-projects/keep-clam.git
cd keep-clam
go build -o clam ./cmd/clam
```

Start clam mode:

```bash
clam enable --timeout 4h
```

If your Mac still sleeps when the lid closes, use force mode:

```bash
clam trust
clam enable --force
```

Check the shell from another terminal:

```bash
clam status
clam status --json
```

Then run the built-in closed-lid test:

```bash
clam test
```

Stop there. You will know whether `clam` is active, whether the lid is open or closed, what power source macOS reports, which helper processes the worker is holding, and whether the Mac actually stayed awake while closed.

## What It Holds

| Surface | Mechanism | Why it exists |
|---|---|---|
| Idle system sleep | IOKit `PreventUserIdleSystemSleep` assertion | Public macOS power assertion held by the worker process. |
| Idle sleep helpers | `caffeinate -ims` | Uses Apple's built-in CLI to hold idle, disk, and system sleep assertions. |
| No-idle fallback | `pmset noidle` | Keeps another public assertion path alive and supervised. |
| Persistence | User `launchd` agent | Restarts the worker at login while clam mode is enabled. |
| Lid visibility | `ioreg -r -k AppleClamshellState` | Reports whether macOS currently sees the lid as open or closed. |
| Local status | JSON state file | Tracks enable time, timeout, helper PIDs, heartbeat, lid state, and warnings. |
| Force mode | `sudo pmset -a disablesleep 1` | Optional global sleep-disable override for Macs that ignore normal assertions on lid close. |
| Trust setup | `/etc/sudoers.d/keep-clam` | Optional narrow sudoers rule for the two exact `pmset disablesleep` commands. |

`clam` does not inspect your processes and does not try to guess which agent matters. It holds the Mac awake; you decide what deserves the shell.

## The Clam Loop

`clam` is a process, not a one-shot flag:

```text
clam enable starts
  -> timeout, force mode, and safety warning are parsed
  -> trust mode, if installed, lets sudo run the exact pmset command without a password
  -> force mode records the old disablesleep value and sets disablesleep=1 with sudo
  -> state is written under ~/Library/Application Support/keep-clam
  -> a user launchd plist is installed under ~/Library/LaunchAgents
  -> launchd starts `clam daemon`
  -> daemon creates an IOKit power assertion
  -> daemon starts caffeinate and pmset noidle
  -> every heartbeat records PIDs, lid state, and worker health
  -> timeout or disable unloads launchd and removes user state
  -> disable restores disablesleep when force mode was used
```

Every enabled session gets a local heartbeat. If `caffeinate` or `pmset noidle` exits, the daemon tries to start it again. If the timeout expires, `clam` disables itself instead of leaving a half-forgotten helper behind.

## Commands

| Command | What it does |
|---|---|
| `clam enable [--timeout 4h]` | Installs and starts the user launchd worker with normal user-space assertions. |
| `clam enable --force` | Uses sudo to set global `pmset disablesleep=1`, then starts the worker. |
| `clam disable` | Stops the worker, terminates helper processes, removes the plist, and removes local state. |
| `clam status` | Shows active state, lid state, power source, duration, helper PIDs, heartbeat, and warnings. |
| `clam status --json` | Emits the same state as JSON for scripts. |
| `clam test [--duration 2m]` | Runs a guided closed-lid heartbeat test and reports the largest pause. |
| `clam toggle [--timeout 4h]` | Enables if inactive, disables if active. |
| `clam trust` | Installs a narrow sudoers rule so force mode can toggle `disablesleep` without repeated password prompts. |
| `clam untrust` | Removes that sudoers rule. |
| `clam uninstall` | Runs the same cleanup path as disable and tells you to remove the binary. |

Timeouts accept Go-style durations such as `30m`, `4h`, and `90m`, plus day values like `1d`. Force mode intentionally does not accept `--timeout` yet because restoring a sudo-only global setting from a background user LaunchAgent is not reliable without a privileged helper.

## Safety Model

Clam mode can make a laptop run warm because work continues while airflow is worse than it is with the lid open. Prefer AC power, a hard surface, and a timeout. Do not put the machine in a bag while clam mode is active.

```bash
clam enable --timeout 2h
```

Normal operation does not require `sudo`. `clam` uses a user launch agent and user-space assertions so disable and uninstall stay reversible.

Force mode does require `sudo` because it changes the global macOS power setting behind `pmset disablesleep`. Use it only when normal mode fails your heartbeat test, and run `clam disable` when you are done.

To avoid repeated password prompts, run:

```bash
clam trust
```

That installs `/etc/sudoers.d/keep-clam` with permission for your user to run only these exact commands without a password:

```text
/usr/bin/pmset -a disablesleep 1
/usr/bin/pmset -a disablesleep 0
```

Remove it with:

```bash
clam untrust
```

## Limits Of The Shell

Closing a Mac laptop lid is not only idle sleep. On recent Apple Silicon Macs, lid-close sleep can be enforced below the layer public user-space tools control, especially without external-display-style hardware.

That means normal mode is best effort. If the machine still sleeps when fully closed, try `clam enable --force`. If force mode still fails on your hardware or macOS version, common workarounds include:

- Use Apple's supported closed-display setup with external power, display, keyboard, and pointing device.
- Use a reputable dummy USB-C or HDMI display dongle so macOS enters an external-display-style clamshell mode.
- Keep the lid slightly open on hardware that refuses software-only closed-lid wakefulness.

`clam` does not use private IOKit lid-close SPI. It chooses boring public mechanisms, an explicit privileged override, honest status, and clean rollback.

## Uninstall

```bash
clam uninstall
```

Then remove the binary from wherever you installed it, such as `$(go env GOPATH)/bin/clam`.

## Development

```bash
go test ./...
go vet ./...
go run ./cmd/clam status
```

This project is macOS-first. Non-macOS builds are only stubs for clear error messages.
