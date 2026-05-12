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
		name string
		args []string
		want time.Duration
	}{
		{name: "empty", args: nil},
		{name: "separate timeout", args: []string{"--timeout", "4h"}, want: 4 * time.Hour},
		{name: "short timeout", args: []string{"-t", "90m"}, want: 90 * time.Minute},
		{name: "equals timeout", args: []string{"--timeout=1d"}, want: 24 * time.Hour},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseEnableArgs(tt.args)
			if err != nil {
				t.Fatalf("parseEnableArgs(%v) returned error: %v", tt.args, err)
			}
			if got != tt.want {
				t.Fatalf("parseEnableArgs(%v) = %s, want %s", tt.args, got, tt.want)
			}
		})
	}
}

func TestParseEnableArgsRejectsBadInput(t *testing.T) {
	tests := [][]string{
		{"--timeout"},
		{"--timeout", "0"},
		{"--timeout=nope"},
		{"--pearl"},
	}

	for _, args := range tests {
		if _, err := parseEnableArgs(args); err == nil {
			t.Fatalf("parseEnableArgs(%v) returned nil error", args)
		}
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
			Enabled:        true,
			EnabledAt:      &enabledAt,
			ExpiresAt:      &expiresAt,
			DaemonPID:      100,
			CaffeinatePID:  101,
			PMSetNoIdlePID: 102,
			IOKitAssertion: true,
			LastLidClosed:  &closed,
			LastHeartbeat:  &heartbeat,
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
}

func fixedNow() time.Time {
	return time.Date(2026, 5, 12, 17, 0, 0, 0, time.UTC)
}
