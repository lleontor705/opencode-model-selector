package opencode

import (
	"os"
	"os/exec"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOSProcessStarterLifecycle(t *testing.T) {
	originalPrepare := prepareProcessTree
	originalTerminate := terminateProcessTree
	t.Cleanup(func() {
		prepareProcessTree = originalPrepare
		terminateProcessTree = originalTerminate
	})

	var prepared atomic.Int32
	var terminated atomic.Int32
	prepareProcessTree = func(*exec.Cmd) error {
		prepared.Add(1)
		return nil
	}
	terminateProcessTree = func(cmd *exec.Cmd) error {
		terminated.Add(1)
		return cmd.Process.Kill()
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestProcessHelper", "--", "wait")
	cmd.Env = append(os.Environ(), "GO_WANT_PROCESS_HELPER=1")
	process, err := (OSProcessStarter{}).Start(cmd)
	require.NoError(t, err)
	assert.EqualValues(t, 1, prepared.Load())

	require.NoError(t, process.Terminate())
	require.NoError(t, process.Terminate(), "termination must be idempotent")
	assert.EqualValues(t, 1, terminated.Load(), "the process tree must only be targeted once")
	require.Error(t, process.Wait(), "a forcefully terminated process should not report success")
}

func TestTerminateAfterWaitIsSafe(t *testing.T) {
	originalPrepare := prepareProcessTree
	originalTerminate := terminateProcessTree
	t.Cleanup(func() {
		prepareProcessTree = originalPrepare
		terminateProcessTree = originalTerminate
	})

	var terminated atomic.Int32
	prepareProcessTree = func(*exec.Cmd) error { return nil }
	terminateProcessTree = func(*exec.Cmd) error {
		terminated.Add(1)
		return nil
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestProcessHelper", "--", "exit")
	cmd.Env = append(os.Environ(), "GO_WANT_PROCESS_HELPER=1")
	process, err := (OSProcessStarter{}).Start(cmd)
	require.NoError(t, err)
	require.NoError(t, process.Wait())
	require.NoError(t, process.Terminate())
	assert.Zero(t, terminated.Load(), "an observed exited process must never be targeted")
}

func TestStartFailureDoesNotReturnProcess(t *testing.T) {
	originalPrepare := prepareProcessTree
	t.Cleanup(func() { prepareProcessTree = originalPrepare })
	prepareProcessTree = func(*exec.Cmd) error { return assert.AnError }

	process, err := (OSProcessStarter{}).Start(exec.Command(os.Args[0]))
	require.ErrorIs(t, err, assert.AnError)
	assert.Nil(t, process)
}

func TestProcessHelper(t *testing.T) {
	if os.Getenv("GO_WANT_PROCESS_HELPER") != "1" {
		return
	}
	for i, arg := range os.Args {
		if arg != "--" || i+1 >= len(os.Args) {
			continue
		}
		if os.Args[i+1] == "wait" {
			time.Sleep(30 * time.Second)
		}
		return
	}
}
