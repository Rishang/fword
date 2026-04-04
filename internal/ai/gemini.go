// gemini.go — Google Gemini provider via generateContent REST API
package ai

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/Rishang/fk/internal/config"
	"github.com/Rishang/fk/internal/logger"
)

type Gemini struct {
	cfg    *config.Config
	client *http.Client
}

func NewGemini(cfg *config.Config) *Gemini {
	return &Gemini{cfg: cfg, client: &http.Client{Timeout: 15 * time.Second}}
}

type geminiRequest struct {
	SystemInstruction *geminiContent  `json:"system_instruction,omitempty"`
	Contents          []geminiContent `json:"contents"`
	GenerationConfig  geminiGenConfig `json:"generationConfig"`
}

type geminiContent struct {
	Parts []geminiPart `json:"parts"`
	Role  string       `json:"role,omitempty"`
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiGenConfig struct {
	MaxOutputTokens int `json:"maxOutputTokens"`
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error,omitempty"`
}

func (g *Gemini) Query(ctx context.Context, req *Request) (*Suggestion, error) {
	model := g.cfg.Model
	if model == "" {
		model = "gemini-1.5-flash"
	}

	system := SystemPrompt()
	user := UserPrompt(req)
	logger.Debug("gemini input", "system_tokens", (len(system)+3)/4, "user_tokens", (len(user)+3)/4, "total_tokens", (len(system)+len(user)+3)/4)
	logger.Debug("gemini system prompt", "content", system)
	logger.Debug("gemini user prompt", "content", user)

	body := geminiRequest{
		SystemInstruction: &geminiContent{
			Parts: []geminiPart{{Text: system}},
		},
		Contents: []geminiContent{
			{Role: "user", Parts: []geminiPart{{Text: user}}},
		},
		GenerationConfig: geminiGenConfig{MaxOutputTokens: g.cfg.MaxTokens},
	}

	url := fmt.Sprintf("%s/v1beta/models/%s:generateContent?key=%s",
		g.cfg.DefaultBaseURL(), model, g.cfg.APIKey)

	var gr geminiResponse
	if _, err := doJSON(ctx, g.client, url, nil, body, &gr); err != nil {
		return nil, fmt.Errorf("gemini: %w", err)
	}
	if gr.Error != nil {
		return nil, fmt.Errorf("gemini API error [%s]: %s", gr.Error.Status, gr.Error.Message)
	}
	if len(gr.Candidates) == 0 || len(gr.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("gemini: empty response")
	}
	out := gr.Candidates[0].Content.Parts[0].Text
	logger.Debug("gemini output", "tokens", (len(out)+3)/4, "content", out)
	return ParseSuggestion(out), nil
}
