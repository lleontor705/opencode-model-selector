package opencode

import (
	"net/http"
	"os/exec"
	"time"
)

const defaultRuntimeProbeTimeout = 10 * time.Second

// NewRuntimeAgentProbe returns the production runtime probe. Its fields remain
// injectable so callers can replace process and HTTP boundaries in tests.
func NewRuntimeAgentProbe() RuntimeAgentProbe {
	return RuntimeAgentProbe{
		Starter:    OSProcessStarter{},
		Executable: "opencode",
		Timeout:    defaultRuntimeProbeTimeout,
		NewClient: func(baseURL string) (RuntimeAgentClient, error) {
			return NewAgentClient(baseURL, http.DefaultClient)
		},
	}
}

func serveCommand(executable string) *exec.Cmd {
	return exec.Command(executable, "serve", "--hostname", "127.0.0.1", "--port", "0")
}
