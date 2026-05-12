package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jethros-projects/keep-clam-cli/internal/clam"
)

func TestParseEnableArgs(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		want      time.Duration
		wantForce bool
	}{
		{name: "empty", args: nil},
		{name: "separate timeout", args: []string{"--timeout", "4h"}, want: 4 * time.Hour},
		{name: "short timeout", args: []string{"-t", "90m"}, want: 90 * time.Minute},
		{name: "equals timeout", args: []string{"--timeout=1d"}, want: 24 * time.Hour},
		{name: "force", args: []string{"--force"}, wantForce: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseEnableArgs(tt.args)
			if err != nil {
				t.Fatalf("parseEnableArgs(%v) returned error: %v", tt.args, err)
			}
			if got.timeout != tt.want {
				t.Fatalf("timeout = %s, want %s", got.timeout, tt.want)
			}
			if got.forceDisableSleep != tt.wantForce {
				t.Fatalf("forceDisableSleep = %v, want %v", got.forceDisableSleep, tt.wantForce)
			}
		})
	}
}

func TestParseEnableArgsRejectsBadInput(t *testing.T) {
	tests := [][]string{
		{"--timeout"},
		{"--timeout", "0"},
		{"--timeout=nope"},
		{"--force", "--timeout", "4h"},
		{"--pearl"},
	}

	for _, args := range tests {
		if _, err := parseEnableArgs(args); err == nil {
			t.Fatalf("parseEnableArgs(%v) returned nil error", args)
		}
	}
}

func TestParseTestArgs(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		wantDuration time.Duration
		wantInterval time.Duration
	}{
		{name: "defaults", wantDuration: 2 * time.Minute, wantInterval: 5 * time.Second},
		{name: "duration", args: []string{"--duration", "90s"}, wantDuration: 90 * time.Second, wantInterval: 5 * time.Second},
		{name: "interval", args: []string{"--interval", "10s"}, wantDuration: 2 * time.Minute, wantInterval: 10 * time.Second},
		{name: "equals", args: []string{"--duration=3m", "--interval=15s"}, wantDuration: 3 * time.Minute, wantInterval: 15 * time.Second},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseTestArgs(tt.args)
			if err != nil {
				t.Fatalf("parseTestArgs(%v) returned error: %v", tt.args, err)
			}
			if got.duration != tt.wantDuration {
				t.Fatalf("duration = %s, want %s", got.duration, tt.wantDuration)
			}
			if got.interval != tt.wantInterval {
				t.Fatalf("interval = %s, want %s", got.interval, tt.wantInterval)
			}
		})
	}
}

func TestParseTestArgsRejectsBadInput(t *testing.T) {
	tests := [][]string{
		{"--duration"},
		{"--duration", "0"},
		{"--interval"},
		{"--interval", "0s"},
		{"--duration", "5s", "--interval", "5s"},
		{"--pearl"},
	}

	for _, args := range tests {
		if _, err := parseTestArgs(args); err == nil {
			t.Fatalf("parseTestArgs(%v) returned nil error", args)
		}
	}
}

func TestAnalyzeHeartbeats(t *testing.T) {
	start := fixedNow()
	pass := analyzeHeartbeats([]time.Time{
		start,
		start.Add(5 * time.Second),
		start.Add(10 * time.Second),
		start.Add(16 * time.Second),
	}, 5*time.Second)
	if !pass.Passed {
		t.Fatalf("expected pass, got %#v", pass)
	}

	fail := analyzeHeartbeats([]time.Time{
		start,
		start.Add(5 * time.Second),
		start.Add(2 * time.Minute),
	}, 5*time.Second)
	if fail.Passed {
		t.Fatalf("expected fail, got %#v", fail)
	}
	if fail.MaxGap != 115*time.Second {
		t.Fatalf("MaxGap = %s, want 1m55s", fail.MaxGap)
	}
}

func TestRunPrintsHelpWithoutCommand(t *testing.T) {
	var stdout bytes.Buffer
	err := Run(context.Background(), []string{"clam"}, Env{Stdout: &stdout, Now: fixedNow, Home: t.TempDir()})
	if err != nil {
		t.Fatalf("Run help returned error: %v", err)
	}
	if !strings.Contains(stdout.String(), "Keep Clam and Carry On") {
		t.Fatalf("help output missing tagline:\n%s", stdout.String())
	}
}

func TestRunRejectsUnknownCommand(t *testing.T) {
	var stdout bytes.Buffer
	err := Run(context.Background(), []string{"clam", "pearl"}, Env{Stdout: &stdout, Now: fixedNow, Home: t.TempDir()})
	if err == nil {
		t.Fatal("Run unknown command returned nil error")
	}
	if !strings.Contains(err.Error(), "unknown command") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPrintStatusActiveClosedLid(t *testing.T) {
	var stdout bytes.Buffer
	enabledAt := fixedNow().Add(-2 * time.Hour)
	expiresAt := fixedNow().Add(30 * time.Minute)
	heartbeat := fixedNow().Add(-5 * time.Second)
	closed := true

	printStatus(&stdout, clam.Status{
		Active: true,
		State: clam.RuntimeState{
			Enabled:              true,
			EnabledAt:            &enabledAt,
			ExpiresAt:            &expiresAt,
			DaemonPID:            100,
			CaffeinatePID:        101,
			PMSetNoIdlePID:       102,
			IOKitAssertion:       true,
			ForceDisableSleep:    true,
			PreviousDisableSleep: intPtr(0),
			LastLidClosed:        &closed,
			LastHeartbeat:        &heartbeat,
		},
		Lid:               clam.LidState{Known: true, Closed: true},
		PowerSource:       "AC Power",
		LaunchAgentLoaded: true,
	}, fixedNow())

	output := stdout.String()
	for _, want := range []string{
		"Clam mode: active",
		"Lid state:",
		"closed",
		"Power source:",
		"AC Power",
		"Launch agent:",
		"loaded",
		"Active for:",
		"2 hours",
		"Daemon PID:",
		"100",
		"Force mode:",
		"previous disablesleep=0",
		"The clam is shut",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("status output missing %q:\n%s", want, output)
		}
	}
}

func TestPrintStatusInactiveHint(t *testing.T) {
	var stdout bytes.Buffer
	printStatus(&stdout, clam.Status{}, fixedNow())

	output := stdout.String()
	if !strings.Contains(output, "Clam mode: inactive") {
		t.Fatalf("status output missing inactive state:\n%s", output)
	}
	if !strings.Contains(output, "clam enable --timeout 4h") {
		t.Fatalf("status output missing enable hint:\n%s", output)
	}
	if !strings.Contains(output, "clam enable --force") {
		t.Fatalf("status output missing force hint:\n%s", output)
	}
}

func fixedNow() time.Time {
	return time.Date(2026, 5, 12, 17, 0, 0, 0, time.UTC)
}

func intPtr(value int) *int {
	return &value
}
