package opencode

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) Do(req *http.Request) (*http.Response, error) {
	return f(req)
}

type trackingBody struct {
	io.Reader
	closed bool
}

func (b *trackingBody) Close() error {
	b.closed = true
	return nil
}

func TestAgentClientAgentsMapsRuntimeFieldsAndEncodesDirectory(t *testing.T) {
	const directory = `C:\work trees\project & one?x=y`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want GET", r.Method)
		}
		if r.URL.Path != "/agent" {
			t.Errorf("path = %q, want /agent", r.URL.Path)
		}
		if got := r.URL.Query().Get("directory"); got != directory {
			t.Errorf("directory = %q, want %q", got, directory)
		}
		if strings.Contains(r.URL.RawQuery, " ") || strings.Contains(r.URL.RawQuery, "& one") {
			t.Errorf("query was not safely encoded: %q", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `[
			{"name":"build","mode":"primary","native":true,"hidden":null,"description":"ignore me","options":{"x":1},"future":"ignored"},
			{"name":"review","mode":"subagent","native":false,"hidden":true},
			{"name":"general","mode":"all","native":false,"hidden":false}
		]`)
	}))
	defer server.Close()

	client, err := NewAgentClient(server.URL, server.Client())
	if err != nil {
		t.Fatalf("NewAgentClient() error = %v", err)
	}
	agents, err := client.Agents(context.Background(), directory)
	if err != nil {
		t.Fatalf("Agents() error = %v", err)
	}
	if len(agents) != 3 {
		t.Fatalf("len(agents) = %d, want 3", len(agents))
	}
	if got := agents[0]; got.Name != "build" || got.Mode != "primary" || !got.Native || got.Hidden != nil {
		t.Errorf("agents[0] = %#v", got)
	}
	if got := agents[1]; got.Name != "review" || got.Mode != "subagent" || got.Native || got.Hidden == nil || !*got.Hidden {
		t.Errorf("agents[1] = %#v", got)
	}
	if got := agents[2]; got.Hidden == nil || *got.Hidden {
		t.Errorf("agents[2] hidden = %v, want pointer to false", got.Hidden)
	}
}

func TestAgentClientAgentsRejectsNonSuccessAndClosesBody(t *testing.T) {
	body := &trackingBody{Reader: strings.NewReader("upstream unavailable")}
	client, err := NewAgentClient("http://127.0.0.1:4096", roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusServiceUnavailable, Status: "503 Service Unavailable", Body: body}, nil
	}))
	if err != nil {
		t.Fatalf("NewAgentClient() error = %v", err)
	}

	_, err = client.Agents(context.Background(), "project")
	if err == nil || !strings.Contains(err.Error(), "503") {
		t.Fatalf("Agents() error = %v, want status error", err)
	}
	if !body.closed {
		t.Error("response body was not closed")
	}
}

func TestAgentClientAgentsRejectsMalformedOrOversizedJSON(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "malformed", body: `[{"name":`},
		{name: "trailing JSON", body: `[] {}`},
		{name: "oversized", body: strings.Repeat(" ", maxAgentResponseBytes+1)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewAgentClient("http://127.0.0.1:4096", roundTripFunc(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(tt.body))}, nil
			}))
			if err != nil {
				t.Fatalf("NewAgentClient() error = %v", err)
			}
			if _, err := client.Agents(context.Background(), "project"); err == nil {
				t.Fatal("Agents() error = nil")
			}
		})
	}
}

func TestAgentClientAgentsPropagatesContextCancellation(t *testing.T) {
	transportCalled := make(chan struct{})
	client, err := NewAgentClient("http://127.0.0.1:4096", roundTripFunc(func(req *http.Request) (*http.Response, error) {
		close(transportCalled)
		<-req.Context().Done()
		return nil, req.Context().Err()
	}))
	if err != nil {
		t.Fatalf("NewAgentClient() error = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := client.Agents(ctx, "project")
		done <- err
	}()
	<-transportCalled
	cancel()

	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("Agents() error = %v, want context.Canceled", err)
	}
}

func TestNewAgentClientRejectsInvalidBaseURL(t *testing.T) {
	for _, baseURL := range []string{"", "://bad", "/relative"} {
		if _, err := NewAgentClient(baseURL, http.DefaultClient); err == nil {
			t.Errorf("NewAgentClient(%q) error = nil", baseURL)
		}
	}
}
