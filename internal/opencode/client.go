package opencode

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const maxAgentResponseBytes = 1 << 20

// RuntimeAgent is the product-relevant subset of an agent returned by the
// opencode runtime. Hidden is a pointer so JSON null remains distinguishable
// from an explicit false value.
type RuntimeAgent struct {
	Name   string `json:"name"`
	Mode   string `json:"mode"`
	Native bool   `json:"native"`
	Hidden *bool  `json:"hidden"`
}

// HTTPClient is the request boundary used by AgentClient. *http.Client
// satisfies it, while callers and tests may inject another implementation.
type HTTPClient interface {
	Do(*http.Request) (*http.Response, error)
}

// AgentClient reads structured agent metadata from an opencode server.
type AgentClient struct {
	baseURL    *url.URL
	httpClient HTTPClient
}

// NewAgentClient creates a client rooted at baseURL.
func NewAgentClient(baseURL string, httpClient HTTPClient) (*AgentClient, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("parse opencode server URL: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("parse opencode server URL: absolute URL required")
	}
	if httpClient == nil {
		return nil, fmt.Errorf("create agent client: HTTP client is required")
	}

	return &AgentClient{baseURL: parsed, httpClient: httpClient}, nil
}

// Agents gets the runtime agent catalog for directory.
func (c *AgentClient) Agents(ctx context.Context, directory string) ([]RuntimeAgent, error) {
	endpoint := *c.baseURL
	endpoint.Path = strings.TrimRight(endpoint.Path, "/") + "/agent"
	endpoint.RawPath = ""
	query := endpoint.Query()
	query.Set("directory", directory)
	endpoint.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create agent request: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request runtime agents: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("request runtime agents: unexpected HTTP status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxAgentResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read runtime agents response: %w", err)
	}
	if len(data) > maxAgentResponseBytes {
		return nil, fmt.Errorf("read runtime agents response: exceeds %d bytes", maxAgentResponseBytes)
	}

	var agents []RuntimeAgent
	if err := json.Unmarshal(data, &agents); err != nil {
		return nil, fmt.Errorf("decode runtime agents response: %w", err)
	}
	return agents, nil
}
