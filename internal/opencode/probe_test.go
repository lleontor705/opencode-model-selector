package opencode

import (
	"context"
	"errors"
	"io"
	"os/exec"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type probeProcess struct {
	wait      chan error
	terminate atomic.Int32
	onStop    func()
}

func (p *probeProcess) Wait() error { return <-p.wait }
func (p *probeProcess) Terminate() error {
	if p.terminate.Add(1) == 1 && p.onStop != nil {
		p.onStop()
	}
	return nil
}

type probeStarter struct {
	start func(*exec.Cmd) (Process, error)
}

func (s probeStarter) Start(cmd *exec.Cmd) (Process, error) { return s.start(cmd) }

func TestRuntimeAgentProbeDiscoversFragmentedURLAndGetsStructuredAgents(t *testing.T) {
	process := &probeProcess{wait: make(chan error, 1)}
	var gotCommand []string
	starter := probeStarter{start: func(cmd *exec.Cmd) (Process, error) {
		gotCommand = append([]string{cmd.Path}, cmd.Args[1:]...)
		go func() {
			_, _ = io.WriteString(cmd.Stderr, "warning: see http://example.com:80\n")
			_, _ = io.WriteString(cmd.Stdout, "opencode server list")
			_, _ = io.WriteString(cmd.Stdout, "ening on http://127.0.0.1:")
			_, _ = io.WriteString(cmd.Stdout, "43210\n")
		}()
		return process, nil
	}}

	var baseURL, directory string
	probe := RuntimeAgentProbe{
		Starter:    starter,
		Executable: "custom-opencode",
		Timeout:    time.Second,
		NewClient: func(got string) (RuntimeAgentClient, error) {
			baseURL = got
			return runtimeAgentClientFunc(func(_ context.Context, gotDirectory string) ([]RuntimeAgent, error) {
				directory = gotDirectory
				return []RuntimeAgent{{Name: "build", Mode: "primary", Native: true}}, nil
			}), nil
		},
	}

	agents, err := probe.Agents(context.Background(), `C:\work tree`)
	if err != nil {
		t.Fatalf("Agents() error = %v", err)
	}
	if len(agents) != 1 || agents[0].Name != "build" {
		t.Fatalf("Agents() = %#v", agents)
	}
	if got := strings.Join(gotCommand, " "); got != "custom-opencode serve --hostname 127.0.0.1 --port 0" {
		t.Errorf("command = %q", got)
	}
	if baseURL != "http://127.0.0.1:43210" {
		t.Errorf("base URL = %q", baseURL)
	}
	if directory != `C:\work tree` {
		t.Errorf("directory = %q", directory)
	}
	if got := process.terminate.Load(); got != 1 {
		t.Errorf("cleanup count = %d, want 1", got)
	}
}

func TestRuntimeAgentProbeRejectsUntrustedURLsThenAcceptsLoopback(t *testing.T) {
	process := &probeProcess{wait: make(chan error, 1)}
	starter := writingProbeStarter(process,
		"opencode server listening on http://example.com:443\n",
		"opencode server listening on http://127.0.0.1:notaport\n",
		"opencode server listening on http://127.0.0.1:4567\n",
	)
	probe := RuntimeAgentProbe{
		Starter: starter, Executable: "opencode", Timeout: time.Second,
		NewClient: func(baseURL string) (RuntimeAgentClient, error) {
			if baseURL != "http://127.0.0.1:4567" {
				t.Fatalf("NewClient(%q)", baseURL)
			}
			return runtimeAgentClientFunc(func(context.Context, string) ([]RuntimeAgent, error) { return nil, nil }), nil
		},
	}
	if _, err := probe.Agents(context.Background(), "."); err != nil {
		t.Fatalf("Agents() error = %v", err)
	}
}

func TestRuntimeAgentProbeReturnsEarlyExitAndCleansUp(t *testing.T) {
	process := &probeProcess{wait: make(chan error, 1)}
	process.wait <- errors.New("exit status 1")
	probe := RuntimeAgentProbe{Starter: writingProbeStarter(process, "failed to bind\n"), Executable: "opencode", Timeout: time.Second, NewClient: unusedClientFactory}
	_, err := probe.Agents(context.Background(), ".")
	if err == nil || !strings.Contains(err.Error(), "exited before") {
		t.Fatalf("Agents() error = %v", err)
	}
	if got := process.terminate.Load(); got != 1 {
		t.Errorf("cleanup count = %d, want 1", got)
	}
}

func TestRuntimeAgentProbeTimeoutAndCancellationCleanUp(t *testing.T) {
	tests := []struct {
		name    string
		context func() (context.Context, context.CancelFunc)
		timeout time.Duration
		want    error
	}{
		{name: "timeout", context: func() (context.Context, context.CancelFunc) { return context.Background(), func() {} }, timeout: 20 * time.Millisecond, want: context.DeadlineExceeded},
		{name: "cancellation", context: func() (context.Context, context.CancelFunc) { return context.WithCancel(context.Background()) }, timeout: time.Second, want: context.Canceled},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			process := &probeProcess{wait: make(chan error, 1)}
			ctx, cancel := tt.context()
			if tt.name == "cancellation" {
				cancel()
			} else {
				defer cancel()
			}
			probe := RuntimeAgentProbe{Starter: writingProbeStarter(process), Executable: "opencode", Timeout: tt.timeout, NewClient: unusedClientFactory}
			_, err := probe.Agents(ctx, ".")
			if !errors.Is(err, tt.want) {
				t.Fatalf("Agents() error = %v, want %v", err, tt.want)
			}
			if got := process.terminate.Load(); got != 1 {
				t.Errorf("cleanup count = %d, want 1", got)
			}
		})
	}
}

func TestRuntimeAgentProbeHTTPFailureCleansUp(t *testing.T) {
	process := &probeProcess{wait: make(chan error, 1)}
	httpErr := errors.New("HTTP failed")
	probe := RuntimeAgentProbe{
		Starter:    writingProbeStarter(process, "opencode server listening on http://127.0.0.1:7777\n"),
		Executable: "opencode", Timeout: time.Second,
		NewClient: func(string) (RuntimeAgentClient, error) {
			return runtimeAgentClientFunc(func(context.Context, string) ([]RuntimeAgent, error) { return nil, httpErr }), nil
		},
	}
	_, err := probe.Agents(context.Background(), ".")
	if !errors.Is(err, httpErr) {
		t.Fatalf("Agents() error = %v", err)
	}
	if got := process.terminate.Load(); got != 1 {
		t.Errorf("cleanup count = %d, want 1", got)
	}
}

func writingProbeStarter(process *probeProcess, output ...string) ProcessStarter {
	return probeStarter{start: func(cmd *exec.Cmd) (Process, error) {
		go func() {
			for i, text := range output {
				writer := cmd.Stdout
				if i%2 == 1 {
					writer = cmd.Stderr
				}
				_, _ = io.WriteString(writer, text)
			}
		}()
		return process, nil
	}}
}

type runtimeAgentClientFunc func(context.Context, string) ([]RuntimeAgent, error)

func (f runtimeAgentClientFunc) Agents(ctx context.Context, directory string) ([]RuntimeAgent, error) {
	return f(ctx, directory)
}

func unusedClientFactory(string) (RuntimeAgentClient, error) {
	return runtimeAgentClientFunc(func(context.Context, string) ([]RuntimeAgent, error) { return nil, nil }), nil
}
