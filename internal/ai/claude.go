// claude.go — Anthropic Claude provider using the /v1/messages API
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/user/fword/internal/config"
)

// Claude talks to Anthropic's Messages API
type Claude struct {
	cfg    *config.Config
	client *http.Client
}

func NewClaude(cfg *config.Config) *Claude {
	return &Claude{
		cfg:    cfg,
		client: &http.Client{Timeout: providerHTTPTimeout},
	}
}

// claudeRequest mirrors the Anthropic /v1/messages request body
type claudeRequest struct {
	Model     string           `json:"model"`
	MaxTokens int              `json:"max_tokens"`
	System    string           `json:"system"`
	Messages  []claudeMessage  `json:"messages"`
}

type claudeMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// claudeResponse mirrors the /v1/messages response (simplified)
type claudeResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (c *Claude) Query(ctx context.Context, req *Request) (*Suggestion, error) {
	body := claudeRequest{
		Model:     c.cfg.Model,
		MaxTokens: c.cfg.MaxTokens,
		System:    SystemPrompt(),
		Messages: []claudeMessage{
			{Role: "user", Content: UserPrompt(req)},
		},
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("claude: marshal request: %w", err)
	}

	// Build request to /v1/messages
	url := c.cfg.DefaultBaseURL() + "/v1/messages"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("claude: create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.cfg.APIKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("claude: http: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("claude: read body: %w", err)
	}

	var cr claudeResponse
	if err := json.Unmarshal(data, &cr); err != nil {
		return nil, fmt.Errorf("claude: parse response: %w", err)
	}

	// Surface API-level errors (bad key, rate limit, etc.)
	if cr.Error != nil {
		return nil, fmt.Errorf("claude API error [%s]: %s", cr.Error.Type, cr.Error.Message)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("claude: unexpected status %d: %s", resp.StatusCode, string(data))
	}

	// Extract text from first content block
	for _, block := range cr.Content {
		if block.Type == "text" && block.Text != "" {
			return ParseSuggestion(block.Text), nil
		}
	}

	return nil, fmt.Errorf("claude: empty response")
}
