//go:build darwin

package clam

import (
	"context"
	"strings"
	"testing"
)

func TestSudoersContentForUser(t *testing.T) {
	content, err := sudoersContentForUser("exampleuser")
	if err != nil {
		t.Fatalf("sudoersContentForUser returned error: %v", err)
	}
	for _, want := range []string{
		"exampleuser ALL=(root) NOPASSWD:",
		"/usr/bin/pmset -a disablesleep 1",
		"/usr/bin/pmset -a disablesleep 0",
		"clam untrust",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("sudoers content missing %q:\n%s", want, content)
		}
	}
}

func TestSudoersContentValidatesWithVisudo(t *testing.T) {
	content, err := sudoersContentForUser("exampleuser")
	if err != nil {
		t.Fatalf("sudoersContentForUser returned error: %v", err)
	}
	if err := validateSudoersContent(context.Background(), content); err != nil {
		t.Fatalf("validateSudoersContent returned error: %v", err)
	}
}

func TestSudoersContentRejectsUnsafeUser(t *testing.T) {
	for _, username := range []string{"bad user", "bad,ALL", "bad\nroot"} {
		if _, err := sudoersContentForUser(username); err == nil {
			t.Fatalf("sudoersContentForUser(%q) returned nil error", username)
		}
	}
}
