package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestKeepClamHelpE2E(t *testing.T) {
	bin := buildClam(t)
	output := runClam(t, bin, []string{"help"}, nil)

	for _, want := range []string{
		"Keep Clam and Carry On",
		"clam enable [--timeout 4h]",
		"clam status [--json]",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("help output missing %q:\n%s", want, output)
		}
	}
}

func TestKeepClamInvalidTimeoutE2E(t *testing.T) {
	bin := buildClam(t)
	cmd := exec.Command(bin, "enable", "--timeout", "nope")
	cmd.Env = isolatedEnv(t)
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("invalid timeout exited successfully:\n%s", string(output))
	}
	if !strings.Contains(string(output), "invalid timeout") {
		t.Fatalf("invalid timeout output missing error:\n%s", string(output))
	}
}

func TestKeepClamStatusJSONE2E(t *testing.T) {
	bin := buildClam(t)
	output := runClam(t, bin, []string{"status", "--json"}, isolatedEnv(t))

	var status struct {
		Active bool `json:"active"`
		State  struct {
			Enabled bool `json:"enabled"`
		} `json:"state"`
		Lid struct {
			Known bool `json:"known"`
		} `json:"lid"`
	}
	if err := json.Unmarshal([]byte(output), &status); err != nil {
		t.Fatalf("status output is not JSON: %v\n%s", err, output)
	}
	if status.Active || status.State.Enabled {
		t.Fatalf("fresh isolated HOME should be inactive: %#v\n%s", status, output)
	}
}

func buildClam(t *testing.T) string {
	t.Helper()

	bin := filepath.Join(t.TempDir(), "clam")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}

	cmd := exec.Command(goBin(t), "build", "-o", bin, ".")
	cmd.Env = append(os.Environ(), "CGO_ENABLED=1")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build failed: %v\n%s", err, string(output))
	}
	return bin
}

func runClam(t *testing.T, bin string, args []string, env []string) string {
	t.Helper()
	cmd := exec.Command(bin, args...)
	if env != nil {
		cmd.Env = env
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s failed: %v\n%s", bin, strings.Join(args, " "), err, string(output))
	}
	return string(output)
}

func isolatedEnv(t *testing.T) []string {
	t.Helper()

	home := t.TempDir()
	env := os.Environ()
	env = append(env, "HOME="+home)
	env = append(env, "XDG_CACHE_HOME="+filepath.Join(home, ".cache"))
	return env
}

func goBin(t *testing.T) string {
	t.Helper()

	candidate := filepath.Join(runtime.GOROOT(), "bin", "go")
	if _, err := os.Stat(candidate); err == nil {
		return candidate
	}
	path, err := exec.LookPath("go")
	if err != nil {
		t.Fatalf("go binary not found: %v", err)
	}
	return path
}
