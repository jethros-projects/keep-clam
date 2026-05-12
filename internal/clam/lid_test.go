//go:build darwin

package clam

import "testing"

func TestParseLidState(t *testing.T) {
	closed, err := ParseLidState(`  |   "AppleClamshellState" = Yes`)
	if err != nil {
		t.Fatalf("ParseLidState closed: %v", err)
	}
	if !closed.Known || !closed.Closed {
		t.Fatalf("closed state = %#v", closed)
	}

	open, err := ParseLidState(`  |   "AppleClamshellState" = No`)
	if err != nil {
		t.Fatalf("ParseLidState open: %v", err)
	}
	if !open.Known || open.Closed {
		t.Fatalf("open state = %#v", open)
	}
}

func TestParseLidStateMissing(t *testing.T) {
	state, err := ParseLidState(`no clam here`)
	if err == nil {
		t.Fatalf("expected error, got state %#v", state)
	}
}
