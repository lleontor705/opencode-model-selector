//go:build windows

package opencode

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"syscall"
)

func platformPrepareProcessTree(cmd *exec.Cmd) error {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.CreationFlags |= syscall.CREATE_NEW_PROCESS_GROUP
	return nil
}

func platformTerminateProcessTree(cmd *exec.Cmd) error {
	// The standard library has no Job Object API. taskkill /T scopes forced
	// termination to the PID and descendants that Windows can still observe.
	kill := exec.Command("taskkill", "/PID", strconv.Itoa(cmd.Process.Pid), "/T", "/F")
	output, err := kill.CombinedOutput()
	if err == nil || errors.Is(err, os.ErrProcessDone) {
		return nil
	}
	// taskkill reports an already-exited PID as a localized non-zero error.
	// Process.Kill gives us a locale-independent way to recognize that race.
	if errors.Is(cmd.Process.Kill(), os.ErrProcessDone) {
		return nil
	}
	return fmt.Errorf("terminate process tree %d with taskkill: %w: %s", cmd.Process.Pid, err, output)
}
