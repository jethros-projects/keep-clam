package clam

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultPaths(t *testing.T) {
	home := filepath.Join(string(filepath.Separator), "Users", "test")
	paths := DefaultPaths(home)

	wantStateDir := filepath.Join(home, "Library", "Application Support", "keep-clam")
	if paths.StateDir != wantStateDir {
		t.Fatalf("StateDir = %q, want %q", paths.StateDir, wantStateDir)
	}
	if paths.StatePath != filepath.Join(wantStateDir, "state.json") {
		t.Fatalf("StatePath = %q", paths.StatePath)
	}
	if paths.PlistPath != filepath.Join(home, "Library", "LaunchAgents", "com.jethros-projects.keep-clam.plist") {
		t.Fatalf("PlistPath = %q", paths.PlistPath)
	}
}

func TestReadStateMissingFile(t *testing.T) {
	state, err := readState(filepath.Join(t.TempDir(), "missing.json"))
	if err != nil {
		t.Fatalf("readState missing returned error: %v", err)
	}
	if state.Enabled {
		t.Fatalf("missing state should be disabled: %#v", state)
	}
}

func TestWriteStateRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "state.json")
	enabledAt := time.Date(2026, 5, 12, 10, 0, 0, 0, time.UTC)
	expiresAt := enabledAt.Add(4 * time.Hour)
	heartbeat := enabledAt.Add(time.Minute)
	closed := true
	previousDisableSleep := 0

	want := RuntimeState{
		Enabled:              true,
		EnabledAt:            &enabledAt,
		TimeoutSeconds:       int64((4 * time.Hour).Seconds()),
		ExpiresAt:            &expiresAt,
		BinaryPath:           "/tmp/clam",
		DaemonPID:            11,
		CaffeinatePID:        12,
		PMSetNoIdlePID:       13,
		IOKitAssertion:       true,
		ForceDisableSleep:    true,
		PreviousDisableSleep: &previousDisableSleep,
		LastLidClosed:        &closed,
		LastHeartbeat:        &heartbeat,
		LastError:            "test warning",
	}

	if err := writeState(path, want); err != nil {
		t.Fatalf("writeState returned error: %v", err)
	}
	got, err := readState(path)
	if err != nil {
		t.Fatalf("readState returned error: %v", err)
	}
	if !got.Enabled || got.BinaryPath != want.BinaryPath || got.DaemonPID != want.DaemonPID {
		t.Fatalf("round trip mismatch: %#v", got)
	}
	if got.EnabledAt == nil || !got.EnabledAt.Equal(enabledAt) {
		t.Fatalf("EnabledAt = %v, want %v", got.EnabledAt, enabledAt)
	}
	if got.ExpiresAt == nil || !got.ExpiresAt.Equal(expiresAt) {
		t.Fatalf("ExpiresAt = %v, want %v", got.ExpiresAt, expiresAt)
	}
	if got.LastHeartbeat == nil || !got.LastHeartbeat.Equal(heartbeat) {
		t.Fatalf("LastHeartbeat = %v, want %v", got.LastHeartbeat, heartbeat)
	}
	if got.LastLidClosed == nil || *got.LastLidClosed != closed {
		t.Fatalf("LastLidClosed = %v, want %v", got.LastLidClosed, closed)
	}
}

func TestReadStateRejectsInvalidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(path, []byte("{"), 0o644); err != nil {
		t.Fatalf("write invalid json: %v", err)
	}
	if _, err := readState(path); err == nil {
		t.Fatal("readState invalid JSON returned nil error")
	}
}

func TestWriteStatusJSON(t *testing.T) {
	var out bytes.Buffer
	status := Status{
		Active: true,
		Lid:    LidState{Known: true, Closed: false},
		State:  RuntimeState{Enabled: true},
	}

	if err := WriteStatusJSON(&out, status); err != nil {
		t.Fatalf("WriteStatusJSON returned error: %v", err)
	}
	var decoded Status
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatalf("status JSON did not decode: %v\n%s", err, out.String())
	}
	if !decoded.Active || !decoded.State.Enabled || !decoded.Lid.Known || decoded.Lid.Closed {
		t.Fatalf("decoded status mismatch: %#v", decoded)
	}
}

func TestLidStateDescription(t *testing.T) {
	tests := []struct {
		name string
		lid  LidState
		want string
	}{
		{name: "open", lid: LidState{Known: true}, want: "open"},
		{name: "closed", lid: LidState{Known: true, Closed: true}, want: "closed"},
		{name: "unknown", lid: LidState{}, want: "unknown"},
		{name: "unknown with error", lid: LidState{Error: "ioreg failed"}, want: "unknown (ioreg failed)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.lid.Description(); got != tt.want {
				t.Fatalf("Description() = %q, want %q", got, tt.want)
			}
		})
	}
}
