// openai.go — OpenAI-compatible provider (works for openai AND openrouter)
// OpenRouter exposes the same /v1/chat/completions interface, so one client covers both.
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/user/fword/internal/config"
)

// OpenAI handles both api.openai.com and openrouter.ai
type OpenAI struct {
	cfg    *config.Config
	client *http.Client
}

const providerHTTPTimeout = 15 * time.Second

func NewOpenAI(cfg *config.Config) *OpenAI {
	return &OpenAI{
		cfg:    cfg,
		client: &http.Client{Timeout: providerHTTPTimeout},
	}
}

type openAIRequest struct {
	Model     string          `json:"model"`
	MaxTokens int             `json:"max_tokens"`
	Messages  []openAIMessage `json:"messages"`
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
}

func (o *OpenAI) Query(ctx context.Context, req *Request) (*Suggestion, error) {
	body := openAIRequest{
		Model:     o.cfg.Model,
		MaxTokens: o.cfg.MaxTokens,
		Messages: []openAIMessage{
			// System prompt sets the persona / output format rules
			{Role: "system", Content: SystemPrompt()},
			{Role: "user", Content: UserPrompt(req)},
		},
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("openai: marshal: %w", err)
	}

	url := chatCompletionsURL(o.cfg.DefaultBaseURL())
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("openai: create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+o.cfg.APIKey)

	// OpenRouter requires an HTTP-Referer header (any value works for personal use)
	if o.cfg.Provider == config.ProviderOpenRouter {
		httpReq.Header.Set("HTTP-Referer", "https://github.com/user/fword")
		httpReq.Header.Set("X-Title", "fword")
	}

	resp, err := o.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("openai: http: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("openai: read body: %w", err)
	}

	var or openAIResponse
	if err := json.Unmarshal(data, &or); err != nil {
		return nil, fmt.Errorf("openai: parse response: %w", err)
	}

	if or.Error != nil {
		return nil, fmt.Errorf("openai API error [%s]: %s", or.Error.Type, or.Error.Message)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openai: unexpected status %d: %s", resp.StatusCode, string(data))
	}
	if len(or.Choices) == 0 {
		return nil, fmt.Errorf("openai: empty choices in response")
	}

	return ParseSuggestion(or.Choices[0].Message.Content), nil
}

func chatCompletionsURL(base string) string {
	base = strings.TrimSuffix(strings.TrimSpace(base), "/")
	if strings.HasSuffix(base, "/v1") {
		return base + "/chat/completions"
	}
	return base + "/v1/chat/completions"
}
