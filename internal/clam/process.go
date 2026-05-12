package clam

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func terminateIfCommand(pid int, commandName string) error {
	name := processName(pid)
	if name != "" && !strings.Contains(name, commandName) {
		return nil
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	if err := process.Signal(syscall.SIGTERM); err != nil {
		return err
	}
	for i := 0; i < 20; i++ {
		if !processAlive(pid) {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return process.Kill()
}

func processName(pid int) string {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "/bin/ps", "-p", strconv.Itoa(pid), "-o", "comm=")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return string(bytes.TrimSpace(output))
}

func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return process.Signal(syscall.Signal(0)) == nil
}
