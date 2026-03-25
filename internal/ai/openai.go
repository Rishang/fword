// openai.go — OpenAI-compatible provider (covers openai and openrouter)
// OpenRouter exposes the same /v1/chat/completions interface, so one client covers both.
package ai

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Rishang/fword/internal/config"
)

type OpenAI struct {
	cfg    *config.Config
	client *http.Client
}

func NewOpenAI(cfg *config.Config) *OpenAI {
	return &OpenAI{cfg: cfg, client: &http.Client{Timeout: 15 * time.Second}}
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
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (o *OpenAI) Query(ctx context.Context, req *Request) (*Suggestion, error) {
	body := openAIRequest{
		Model:     o.cfg.Model,
		MaxTokens: o.cfg.MaxTokens,
		Messages: []openAIMessage{
			{Role: "system", Content: SystemPrompt()},
			{Role: "user", Content: UserPrompt(req)},
		},
	}

	headers := map[string]string{
		"Authorization": "Bearer " + o.cfg.APIKey,
	}
	if o.cfg.Provider == config.ProviderOpenRouter {
		headers["HTTP-Referer"] = config.ProjectURL
		headers["X-Title"] = "fword"
	}

	var or openAIResponse
	url := strings.TrimRight(o.cfg.DefaultBaseURL(), "/") + "/chat/completions"
	if _, err := doJSON(ctx, o.client, url, headers, body, &or); err != nil {
		return nil, fmt.Errorf("openai: %w", err)
	}
	if or.Error != nil {
		return nil, fmt.Errorf("openai API error [%s]: %s", or.Error.Type, or.Error.Message)
	}
	if len(or.Choices) == 0 {
		return nil, fmt.Errorf("openai: empty choices in response")
	}
	return ParseSuggestion(or.Choices[0].Message.Content), nil
}
