package opencode

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

const integrationWait = 5 * time.Second

func TestRuntimeAgentProbeEndToEnd(t *testing.T) {
	executable := buildFakeOpenCode(t)
	runtimeData := filepath.Join(repoRoot(t), "testdata", "runtime")

	t.Run("delayed startup, encoded directory, agents, and process-tree cleanup", func(t *testing.T) {
		temp := t.TempDir()
		pidFile := filepath.Join(temp, "root.pid")
		childPIDFile := filepath.Join(temp, "child.pid")
		queryFile := filepath.Join(temp, "query.txt")
		directory := filepath.Join(temp, "work tree & one?x=y")
		t.Setenv("FAKE_OPENCODE_DELAY", "100ms")
		t.Setenv("FAKE_OPENCODE_PID_FILE", pidFile)
		t.Setenv("FAKE_OPENCODE_CHILD_PID_FILE", childPIDFile)
		t.Setenv("FAKE_OPENCODE_QUERY_FILE", queryFile)
		t.Setenv("FAKE_OPENCODE_EXPECT_DIRECTORY", directory)
		t.Setenv("FAKE_OPENCODE_AGENTS_FILE", filepath.Join(runtimeData, "agents.json"))

		probe := realRuntimeProbe(executable, 3*time.Second)
		agents, err := probe.Agents(context.Background(), directory)
		if err != nil {
			t.Fatalf("Agents() error = %v", err)
		}
		if len(agents) != 3 || agents[0].Name != "build" || agents[0].Mode != "primary" || !agents[0].Native || agents[0].Hidden != nil {
			t.Fatalf("Agents() = %#v", agents)
		}
		if agents[1].Hidden == nil || !*agents[1].Hidden || agents[2].Hidden == nil || *agents[2].Hidden {
			t.Fatalf("hidden values were not preserved: %#v", agents)
		}
		rawQuery := waitFile(t, queryFile)
		if want := "directory=" + url.QueryEscape(directory); rawQuery != want {
			t.Errorf("raw query = %q, want %q", rawQuery, want)
		}

		rootPID := readPID(t, pidFile)
		childPID := readPID(t, childPIDFile)
		ensureProcessGone(t, rootPID, true)
		ensureProcessGone(t, childPID, runtime.GOOS != "windows")
	})

	for _, tc := range []struct {
		name    string
		timeout time.Duration
		cancel  bool
		wantErr error
	}{
		{name: "startup timeout", timeout: 250 * time.Millisecond, wantErr: context.DeadlineExceeded},
		{name: "caller cancellation", timeout: 3 * time.Second, cancel: true, wantErr: context.Canceled},
	} {
		t.Run(tc.name, func(t *testing.T) {
			temp := t.TempDir()
			pidFile := filepath.Join(temp, "root.pid")
			t.Setenv("FAKE_OPENCODE_DELAY", "30s")
			t.Setenv("FAKE_OPENCODE_PID_FILE", pidFile)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			result := make(chan error, 1)
			go func() {
				_, err := realRuntimeProbe(executable, tc.timeout).Agents(ctx, temp)
				result <- err
			}()
			rootPID := readPID(t, pidFile)
			defer killPID(rootPID)
			if tc.cancel {
				cancel()
			}
			select {
			case err := <-result:
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("Agents() error = %v, want %v", err, tc.wantErr)
				}
			case <-time.After(integrationWait):
				t.Fatal("Agents() did not return within the cleanup bound")
			}
			ensureProcessGone(t, rootPID, true)
		})
	}
}

func realRuntimeProbe(executable string, timeout time.Duration) RuntimeAgentProbe {
	probe := NewRuntimeAgentProbe()
	probe.Executable = executable
	probe.Timeout = timeout
	probe.NewClient = func(baseURL string) (RuntimeAgentClient, error) {
		return NewAgentClient(baseURL, http.DefaultClient)
	}
	return probe
}

func buildFakeOpenCode(t *testing.T) string {
	t.Helper()
	name := "fake-opencode"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	executable := filepath.Join(t.TempDir(), name)
	cmd := exec.Command("go", "build", "-o", executable, "./testdata/fake-opencode")
	cmd.Dir = repoRoot(t)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build fake opencode: %v\n%s", err, output)
	}
	return executable
}

func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func waitFile(t *testing.T, path string) string {
	t.Helper()
	deadline := time.Now().Add(integrationWait)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(path)
		if err == nil && len(data) != 0 {
			return strings.TrimSpace(string(data))
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("read %s: %v", path, err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", path)
	return ""
}

func readPID(t *testing.T, path string) int {
	t.Helper()
	text := waitFile(t, path)
	pid, err := strconv.Atoi(text)
	if err != nil {
		t.Fatalf("parse PID %q: %v", text, err)
	}
	return pid
}

func ensureProcessGone(t *testing.T, pid int, required bool) {
	t.Helper()
	defer killPID(pid)
	deadline := time.Now().Add(integrationWait)
	for time.Now().Before(deadline) {
		if !processAlive(pid) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	if required {
		t.Fatalf("process %d remained alive after probe cleanup", pid)
	}
	t.Logf("best effort: Windows taskkill did not reap descendant PID %d; cleaning it directly", pid)
}

func processAlive(pid int) bool {
	if runtime.GOOS == "windows" {
		output, err := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/FO", "CSV", "/NH").Output()
		return err == nil && strings.Contains(string(output), fmt.Sprintf("\",\"%d\",", pid))
	}
	return exec.Command("kill", "-0", strconv.Itoa(pid)).Run() == nil
}

func killPID(pid int) {
	if !processAlive(pid) {
		return
	}
	if runtime.GOOS == "windows" {
		_ = exec.Command("taskkill", "/PID", strconv.Itoa(pid), "/T", "/F").Run()
		return
	}
	_ = exec.Command("kill", "-KILL", strconv.Itoa(pid)).Run()
}
