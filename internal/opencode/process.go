package opencode

import (
	"os/exec"
	"sync"
)

// Process is a started child whose completion can be observed and whose
// process tree can be terminated. Terminate may be called more than once.
type Process interface {
	Wait() error
	Terminate() error
}

// ProcessStarter starts a command with platform process-tree isolation. It is
// an interface so the runtime launcher can inject a deterministic fake.
type ProcessStarter interface {
	Start(*exec.Cmd) (Process, error)
}

// OSProcessStarter starts real operating-system processes.
type OSProcessStarter struct{}

var (
	prepareProcessTree   = platformPrepareProcessTree
	terminateProcessTree = platformTerminateProcessTree
)

// Start configures cmd as the root of a process tree before starting it.
func (OSProcessStarter) Start(cmd *exec.Cmd) (Process, error) {
	if err := prepareProcessTree(cmd); err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return &osProcess{cmd: cmd}, nil
}

type osProcess struct {
	cmd *exec.Cmd

	mu       sync.Mutex
	exited   bool
	termErr  error
	termOnce sync.Once
}

func (p *osProcess) Wait() error {
	err := p.cmd.Wait()
	p.mu.Lock()
	p.exited = true
	p.mu.Unlock()
	return err
}

func (p *osProcess) Terminate() error {
	p.termOnce.Do(func() {
		p.mu.Lock()
		defer p.mu.Unlock()
		if p.exited {
			return
		}
		p.termErr = terminateProcessTree(p.cmd)
	})
	return p.termErr
}
