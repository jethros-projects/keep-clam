//go:build darwin

package clam

/*
#cgo LDFLAGS: -framework IOKit -framework CoreFoundation
#include <CoreFoundation/CoreFoundation.h>
#include <IOKit/pwr_mgt/IOPMLib.h>

static IOReturn keepclam_create_assertion(IOPMAssertionID *id) {
	return IOPMAssertionCreateWithName(
		kIOPMAssertionTypePreventUserIdleSystemSleep,
		kIOPMAssertionLevelOn,
		CFSTR("keep-clam: agents are safe inside the shell"),
		id
	);
}

static IOReturn keepclam_release_assertion(IOPMAssertionID id) {
	return IOPMAssertionRelease(id);
}
*/
import "C"

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type powerAssertion struct {
	id     C.IOPMAssertionID
	active bool
}

func acquirePowerAssertion() (*powerAssertion, error) {
	var id C.IOPMAssertionID
	result := C.keepclam_create_assertion(&id)
	if result != C.kIOReturnSuccess {
		return nil, fmt.Errorf("create IOKit power assertion: IOReturn %d", int(result))
	}
	return &powerAssertion{id: id, active: true}, nil
}

func (a *powerAssertion) Release() error {
	if a == nil || !a.active {
		return nil
	}
	result := C.keepclam_release_assertion(a.id)
	a.active = false
	if result != C.kIOReturnSuccess {
		return fmt.Errorf("release IOKit power assertion: IOReturn %d", int(result))
	}
	return nil
}

func (a *powerAssertion) Active() bool {
	return a != nil && a.active
}

func DetectLidState(ctx context.Context) LidState {
	childCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(childCtx, "/usr/sbin/ioreg", "-r", "-k", "AppleClamshellState")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return LidState{Error: strings.TrimSpace(string(output))}
	}
	state, parseErr := ParseLidState(string(output))
	if parseErr != nil {
		state.Error = parseErr.Error()
	}
	return state
}

func ParseLidState(output string) (LidState, error) {
	const key = `"AppleClamshellState"`
	for _, line := range strings.Split(output, "\n") {
		if !strings.Contains(line, key) {
			continue
		}
		switch {
		case strings.Contains(line, "= Yes"):
			return LidState{Known: true, Closed: true, Raw: strings.TrimSpace(line)}, nil
		case strings.Contains(line, "= No"):
			return LidState{Known: true, Closed: false, Raw: strings.TrimSpace(line)}, nil
		default:
			return LidState{Raw: strings.TrimSpace(line)}, fmt.Errorf("unrecognized AppleClamshellState value")
		}
	}
	return LidState{}, fmt.Errorf("AppleClamshellState not found")
}

func DetectPowerSource(ctx context.Context) string {
	childCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	output, err := exec.CommandContext(childCtx, "/usr/bin/pmset", "-g", "batt").CombinedOutput()
	if err != nil {
		return ""
	}
	line := strings.TrimSpace(strings.SplitN(string(output), "\n", 2)[0])
	line = strings.TrimPrefix(line, "Now drawing from ")
	line = strings.Trim(line, "'")
	return line
}
