// Package remote implements a Sandbox that forwards tool calls to a running
// tool-server endpoint over HTTP.
//
// RemoteSandbox is not registered as a driver and is not created via
// hands.OpenSandbox — it is instantiated directly by pool managers
// (e.g. DockerManager.Start) after they start the remote process.
package remote

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/tool"
)

// executeTimeout caps a single tool-call round-trip to the remote tool-server.
const executeTimeout = 120 * time.Second

// RemoteSandbox implements hands.Sandbox by forwarding Execute calls to a
// running tool-server endpoint over HTTP. It does not start or stop the remote
// process — that is the responsibility of the pool manager.
type RemoteSandbox struct {
	endpoint string
	token    string // Bearer token sent as Authorization header; empty = no auth
	client   *http.Client
}

// New creates a RemoteSandbox targeting endpoint (e.g. "http://localhost:49321").
// token is the Bearer secret configured on the tool-server; pass empty string
// for unauthenticated connections (local dev only).
func New(endpoint, token string) *RemoteSandbox {
	return &RemoteSandbox{
		endpoint: endpoint,
		token:    token,
		client:   &http.Client{Timeout: executeTimeout},
	}
}

// HealthEndpoint returns the base URL of the remote tool-server.
// Implementing hands.HealthEndpointer allows pool managers to health-check
// without a type assertion on the concrete type.
func (s *RemoteSandbox) HealthEndpoint() string { return s.endpoint }

// Provision is a no-op: the container already has tools loaded via its binary.
func (s *RemoteSandbox) Provision(_ context.Context, _ []tool.InvokableTool) error {
	return nil
}

// Execute forwards the tool call to the remote tool-server's /execute endpoint.
func (s *RemoteSandbox) Execute(ctx context.Context, name string, input json.RawMessage) (string, error) {
	type execReq struct {
		Name  string `json:"name"`
		Input string `json:"input"`
	}
	type execResp struct {
		Output string `json:"output"`
		Error  string `json:"error,omitempty"`
	}

	body, err := json.Marshal(execReq{Name: name, Input: string(input)})
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		s.endpoint+"/execute", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if s.token != "" {
		req.Header.Set("Authorization", "Bearer "+s.token)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("tool-server request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return "", fmt.Errorf("tool-server HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var result execResp
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}
	if result.Error != "" {
		return "", fmt.Errorf("%s", result.Error)
	}
	return result.Output, nil
}

// Close is a no-op: the pool manager owns the remote process lifecycle.
func (s *RemoteSandbox) Close() error { return nil }
