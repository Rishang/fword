// openai.go — OpenAI-compatible provider (covers openai and openrouter)
// OpenRouter exposes the same /v1/chat/completions interface, so one client covers both.
package ai

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Rishang/fk/internal/config"
	"github.com/Rishang/fk/internal/logger"
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
	system := SystemPrompt()
	user := UserPrompt(req)
	logger.Debug("openai input", "system_tokens", (len(system)+3)/4, "user_tokens", (len(user)+3)/4, "total_tokens", (len(system)+len(user)+3)/4)
	logger.Debug("openai system prompt", "content", system)
	logger.Debug("openai user prompt", "content", user)

	body := openAIRequest{
		Model:     o.cfg.Model,
		MaxTokens: o.cfg.MaxTokens,
		Messages: []openAIMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
	}

	headers := map[string]string{
		"Authorization": "Bearer " + o.cfg.APIKey,
	}
	if o.cfg.Provider == config.ProviderOpenRouter {
		headers["HTTP-Referer"] = config.ProjectURL
		headers["X-Title"] = "fk"
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
	out := or.Choices[0].Message.Content
	logger.Debug("openai output", "tokens", (len(out)+3)/4, "content", out)
	return ParseSuggestion(out), nil
}
