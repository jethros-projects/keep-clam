//go:build darwin

package clam

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"regexp"
	"strings"
	"time"
)

const sudoersPath = "/etc/sudoers.d/keep-clam-cli"

var sudoersUserPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)

func InstallTrust(ctx context.Context) error {
	username, err := currentUsername()
	if err != nil {
		return err
	}
	content, err := sudoersContentForUser(username)
	if err != nil {
		return err
	}
	if err := validateSudoersContent(ctx, content); err != nil {
		return err
	}
	script := `set -e
path="/etc/sudoers.d/keep-clam-cli"
tmp="${path}.tmp"
trap 'rm -f "$tmp"' EXIT
cat > "$tmp"
/usr/sbin/visudo -cf "$tmp"
/usr/sbin/chown root:wheel "$tmp"
/bin/chmod 0440 "$tmp"
/bin/mv "$tmp" "$path"
/usr/sbin/visudo -cf /etc/sudoers
`
	if output, err := runCommandWithInput(ctx, content, "/usr/bin/sudo", "/bin/sh", "-c", script); err != nil {
		return formatCommandError("install sudoers trust", output, err)
	}
	return nil
}

func RemoveTrust(ctx context.Context) error {
	script := `set -e
/bin/rm -f /etc/sudoers.d/keep-clam-cli
/usr/sbin/visudo -cf /etc/sudoers
`
	if output, err := runCommand(ctx, "/usr/bin/sudo", "/bin/sh", "-c", script); err != nil {
		return formatCommandError("remove sudoers trust", output, err)
	}
	return nil
}

func TrustPath() string {
	return sudoersPath
}

func currentUsername() (string, error) {
	if sudoUser := strings.TrimSpace(os.Getenv("SUDO_USER")); sudoUser != "" && sudoUser != "root" {
		return validateSudoersUser(sudoUser)
	}
	current, err := user.Current()
	if err == nil && current.Username != "" {
		username := current.Username
		if index := strings.LastIndex(username, `\`); index >= 0 {
			username = username[index+1:]
		}
		return validateSudoersUser(username)
	}
	if envUser := strings.TrimSpace(os.Getenv("USER")); envUser != "" {
		return validateSudoersUser(envUser)
	}
	return "", fmt.Errorf("find current user for sudoers trust")
}

func validateSudoersUser(username string) (string, error) {
	if !sudoersUserPattern.MatchString(username) {
		return "", fmt.Errorf("unsupported username %q for sudoers trust", username)
	}
	return username, nil
}

func sudoersContentForUser(username string) (string, error) {
	username, err := validateSudoersUser(username)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(`# keep-clam-cli force mode
# Allows %s to toggle only macOS disablesleep for clam --force.
# Remove with: clam untrust
%s ALL=(root) NOPASSWD: /usr/bin/pmset -a disablesleep 1, /usr/bin/pmset -a disablesleep 0
`, username, username), nil
}

func validateSudoersContent(ctx context.Context, content string) error {
	tmp, err := os.CreateTemp("", "keep-clam-sudoers-*")
	if err != nil {
		return err
	}
	path := tmp.Name()
	defer os.Remove(path)
	if _, err := tmp.WriteString(content); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if output, err := runCommand(ctx, "/usr/sbin/visudo", "-cf", path); err != nil {
		return formatCommandError("validate sudoers trust", output, err)
	}
	return nil
}

func runCommandWithInput(ctx context.Context, input string, name string, args ...string) (string, error) {
	childCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(childCtx, name, args...)
	cmd.Stdin = strings.NewReader(input)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func formatCommandError(action string, output string, err error) error {
	output = strings.TrimSpace(output)
	if output == "" {
		return fmt.Errorf("%s: %w", action, err)
	}
	return fmt.Errorf("%s: %w: %s", action, err, output)
}
