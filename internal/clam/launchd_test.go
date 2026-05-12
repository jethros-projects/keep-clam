package clam

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteLaunchAgent(t *testing.T) {
	root := t.TempDir()
	paths := Paths{
		StateDir:   filepath.Join(root, "state"),
		PlistPath:  filepath.Join(root, "LaunchAgents", "com.jethros-projects.keep-clam.plist"),
		StdoutPath: filepath.Join(root, "state", "daemon.out.log"),
		StderrPath: filepath.Join(root, "state", "daemon.err.log"),
	}
	executable := filepath.Join(root, "bin", "clam&helper")

	if err := writeLaunchAgent(paths, executable); err != nil {
		t.Fatalf("writeLaunchAgent returned error: %v", err)
	}
	data, err := os.ReadFile(paths.PlistPath)
	if err != nil {
		t.Fatalf("read plist: %v", err)
	}
	plist := string(data)

	for _, want := range []string{
		"<string>com.jethros-projects.keep-clam</string>",
		"<string>daemon</string>",
		"<key>RunAtLoad</key>",
		"<key>KeepAlive</key>",
		"<key>SuccessfulExit</key>",
		"<false/>",
		"<string>Background</string>",
		paths.StdoutPath,
		paths.StderrPath,
		"clam&amp;helper",
	} {
		if !strings.Contains(plist, want) {
			t.Fatalf("plist missing %q:\n%s", want, plist)
		}
	}
}

func TestPlistEscape(t *testing.T) {
	got := plistEscape(`a&b<c>d"e`)
	want := `a&amp;b&lt;c&gt;d&#34;e`
	if got != want {
		t.Fatalf("plistEscape() = %q, want %q", got, want)
	}
}
