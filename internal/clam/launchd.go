package clam

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func writeLaunchAgent(paths Paths, executable string) error {
	if err := os.MkdirAll(filepath.Dir(paths.PlistPath), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(paths.StateDir, 0o755); err != nil {
		return err
	}
	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>%s</string>
  <key>ProgramArguments</key>
  <array>
    <string>%s</string>
    <string>daemon</string>
  </array>
  <key>RunAtLoad</key>
  <true/>
  <key>KeepAlive</key>
  <dict>
    <key>SuccessfulExit</key>
    <false/>
  </dict>
  <key>ProcessType</key>
  <string>Background</string>
  <key>StandardOutPath</key>
  <string>%s</string>
  <key>StandardErrorPath</key>
  <string>%s</string>
  <key>EnvironmentVariables</key>
  <dict>
    <key>PATH</key>
    <string>/usr/bin:/bin:/usr/sbin:/sbin:/opt/homebrew/bin:/usr/local/bin</string>
  </dict>
</dict>
</plist>
`, plistEscape(LaunchLabel), plistEscape(executable), plistEscape(paths.StdoutPath), plistEscape(paths.StderrPath))
	return os.WriteFile(paths.PlistPath, []byte(plist), 0o644)
}

func plistEscape(value string) string {
	var buf bytes.Buffer
	_ = xml.EscapeText(&buf, []byte(value))
	return buf.String()
}

func restartLaunchAgent(ctx context.Context, plistPath string) error {
	_ = stopLaunchAgent(ctx, plistPath)
	if err := runLaunchctl(ctx, "bootstrap", launchDomain(), plistPath); err != nil {
		return fmt.Errorf("load launch agent: %w", err)
	}
	return nil
}

func stopLaunchAgent(ctx context.Context, plistPath string) error {
	err := runLaunchctl(ctx, "bootout", launchDomain()+"/"+LaunchLabel)
	if err == nil || isLaunchctlNotLoaded(err) {
		return waitForLaunchAgentUnloaded(ctx)
	}
	err = runLaunchctl(ctx, "bootout", launchDomain(), plistPath)
	if err == nil || isLaunchctlNotLoaded(err) {
		return waitForLaunchAgentUnloaded(ctx)
	}
	return err
}

func waitForLaunchAgentUnloaded(ctx context.Context) error {
	for i := 0; i < 20; i++ {
		if !launchAgentLoaded(ctx) {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
	return nil
}

func launchAgentLoaded(ctx context.Context) bool {
	return runLaunchctl(ctx, "print", launchDomain()+"/"+LaunchLabel) == nil
}

func launchDomain() string {
	return "gui/" + strconv.Itoa(os.Getuid())
}

func runLaunchctl(ctx context.Context, args ...string) error {
	childCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(childCtx, "/bin/launchctl", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return launchctlError{Args: args, Output: strings.TrimSpace(string(output)), Err: err}
	}
	return nil
}

type launchctlError struct {
	Args   []string
	Output string
	Err    error
}

func (e launchctlError) Error() string {
	if e.Output == "" {
		return fmt.Sprintf("launchctl %s: %v", strings.Join(e.Args, " "), e.Err)
	}
	return fmt.Sprintf("launchctl %s: %v: %s", strings.Join(e.Args, " "), e.Err, e.Output)
}

func (e launchctlError) Unwrap() error {
	return e.Err
}

func isLaunchctlNotLoaded(err error) bool {
	var lc launchctlError
	if !errors.As(err, &lc) {
		return false
	}
	output := strings.ToLower(lc.Output)
	return strings.Contains(output, "no such process") ||
		strings.Contains(output, "could not find service") ||
		strings.Contains(output, "service is disabled")
}
