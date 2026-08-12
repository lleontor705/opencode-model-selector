package opencode

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const listenMessage = "opencode server listening on "

// RuntimeAgentClient is the structured runtime-agent API consumed by the
// probe. AgentClient satisfies this interface.
type RuntimeAgentClient interface {
	Agents(context.Context, string) ([]RuntimeAgent, error)
}

// RuntimeAgentProbe starts a temporary opencode server and reads its agents.
// Starter, NewClient, and Executable are injectable runtime boundaries.
type RuntimeAgentProbe struct {
	Starter    ProcessStarter
	NewClient  func(baseURL string) (RuntimeAgentClient, error)
	Executable string
	Timeout    time.Duration
}

// Agents starts a bounded temporary server probe for directory.
func (p RuntimeAgentProbe) Agents(ctx context.Context, directory string) ([]RuntimeAgent, error) {
	if p.Starter == nil {
		return nil, errors.New("probe runtime agents: process starter is required")
	}
	if p.NewClient == nil {
		return nil, errors.New("probe runtime agents: client factory is required")
	}
	if p.Executable == "" {
		return nil, errors.New("probe runtime agents: executable is required")
	}
	if p.Timeout <= 0 {
		return nil, errors.New("probe runtime agents: positive timeout is required")
	}

	probeCtx, cancel := context.WithTimeout(ctx, p.Timeout)
	defer cancel()

	stdoutReader, stdoutWriter := io.Pipe()
	stderrReader, stderrWriter := io.Pipe()
	cmd := serveCommand(p.Executable)
	cmd.Stdout = stdoutWriter
	cmd.Stderr = stderrWriter

	process, err := p.Starter.Start(cmd)
	if err != nil {
		closeProbePipes(stdoutReader, stdoutWriter, stderrReader, stderrWriter)
		return nil, fmt.Errorf("start opencode server: %w", err)
	}
	defer func() {
		cancel()
		_ = process.Terminate()
		closeProbePipes(stdoutReader, stdoutWriter, stderrReader, stderrWriter)
	}()

	urls := make(chan string, 2)
	go scanServerURLs(probeCtx, stdoutReader, urls)
	go scanServerURLs(probeCtx, stderrReader, urls)
	exited := make(chan error, 1)
	go func() { exited <- process.Wait() }()

	for {
		select {
		case <-probeCtx.Done():
			return nil, fmt.Errorf("discover opencode server URL: %w", probeCtx.Err())
		case err := <-exited:
			if err == nil {
				err = errors.New("process exited")
			}
			return nil, fmt.Errorf("opencode server exited before announcing a URL: %w", err)
		case candidate := <-urls:
			baseURL, ok := validatedLoopbackURL(candidate)
			if !ok {
				continue
			}
			client, err := p.NewClient(baseURL)
			if err != nil {
				return nil, fmt.Errorf("create runtime agent client: %w", err)
			}
			agents, err := client.Agents(probeCtx, directory)
			if err != nil {
				return nil, fmt.Errorf("get runtime agents: %w", err)
			}
			return agents, nil
		}
	}
}

func scanServerURLs(ctx context.Context, reader io.Reader, found chan<- string) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, listenMessage) {
			continue
		}
		fields := strings.Fields(strings.TrimPrefix(line, listenMessage))
		if len(fields) != 0 {
			select {
			case found <- fields[0]:
			case <-ctx.Done():
				return
			}
		}
	}
}

func validatedLoopbackURL(candidate string) (string, bool) {
	parsed, err := url.Parse(candidate)
	if err != nil || parsed.Scheme != "http" || parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", false
	}
	ip := net.ParseIP(parsed.Hostname())
	if ip == nil || !ip.IsLoopback() {
		return "", false
	}
	port, err := strconv.Atoi(parsed.Port())
	if err != nil || port < 1 || port > 65535 {
		return "", false
	}
	return parsed.String(), true
}

func closeProbePipes(pipes ...io.Closer) {
	for _, pipe := range pipes {
		_ = pipe.Close()
	}
}
