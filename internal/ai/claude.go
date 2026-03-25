// claude.go — Anthropic Claude provider using the /v1/messages API
package ai

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/Rishang/fword/internal/config"
)

type Claude struct {
	cfg    *config.Config
	client *http.Client
}

func NewClaude(cfg *config.Config) *Claude {
	return &Claude{cfg: cfg, client: &http.Client{Timeout: 15 * time.Second}}
}

type claudeRequest struct {
	Model     string          `json:"model"`
	MaxTokens int             `json:"max_tokens"`
	System    string          `json:"system"`
	Messages  []claudeMessage `json:"messages"`
}

type claudeMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

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
		Messages:  []claudeMessage{{Role: "user", Content: UserPrompt(req)}},
	}

	var cr claudeResponse
	if _, err := doJSON(ctx, c.client,
		c.cfg.DefaultBaseURL()+"/v1/messages",
		map[string]string{
			"x-api-key":         c.cfg.APIKey,
			"anthropic-version": "2023-06-01",
		},
		body, &cr,
	); err != nil {
		return nil, fmt.Errorf("claude: %w", err)
	}
	if cr.Error != nil {
		return nil, fmt.Errorf("claude API error [%s]: %s", cr.Error.Type, cr.Error.Message)
	}
	for _, block := range cr.Content {
		if block.Type == "text" && block.Text != "" {
			return ParseSuggestion(block.Text), nil
		}
	}
	return nil, fmt.Errorf("claude: empty response")
}
