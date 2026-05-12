//go:build !darwin

package clam

import (
	"context"
	"errors"
)

type powerAssertion struct{}

func acquirePowerAssertion() (*powerAssertion, error) {
	return nil, errors.New("IOKit power assertions require macOS")
}

func (a *powerAssertion) Release() error {
	return nil
}

func (a *powerAssertion) Active() bool {
	return false
}

func DetectLidState(ctx context.Context) LidState {
	return LidState{Error: "lid state detection requires macOS"}
}

func ParseLidState(output string) (LidState, error) {
	return LidState{}, errors.New("lid state parsing requires macOS")
}

func DetectPowerSource(ctx context.Context) string {
	return ""
}

func ReadDisableSleep(ctx context.Context) (int, error) {
	return 0, errors.New("pmset disablesleep requires macOS")
}

func SetDisableSleep(ctx context.Context, value int, prompt bool) error {
	return errors.New("pmset disablesleep requires macOS")
}

func InstallTrust(ctx context.Context) error {
	return errors.New("clam trust requires macOS")
}

func RemoveTrust(ctx context.Context) error {
	return errors.New("clam untrust requires macOS")
}

func TrustPath() string {
	return ""
}
