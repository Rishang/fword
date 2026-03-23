// gemini.go — Google Gemini provider via generateContent REST API
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

// Gemini talks to Google's generativelanguage.googleapis.com
type Gemini struct {
	cfg    *config.Config
	client *http.Client
}

func NewGemini(cfg *config.Config) *Gemini {
	return &Gemini{
		cfg:    cfg,
		client: &http.Client{Timeout: providerHTTPTimeout},
	}
}

// Gemini REST API request shape
type geminiRequest struct {
	SystemInstruction *geminiContent   `json:"system_instruction,omitempty"`
	Contents          []geminiContent  `json:"contents"`
	GenerationConfig  geminiGenConfig  `json:"generationConfig"`
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

// Gemini REST API response (simplified)
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
	body := geminiRequest{
		// System prompt is passed via system_instruction (supported in Gemini 1.5+)
		SystemInstruction: &geminiContent{
			Parts: []geminiPart{{Text: SystemPrompt()}},
		},
		Contents: []geminiContent{
			{
				Role:  "user",
				Parts: []geminiPart{{Text: UserPrompt(req)}},
			},
		},
		GenerationConfig: geminiGenConfig{
			MaxOutputTokens: g.cfg.MaxTokens,
		},
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("gemini: marshal: %w", err)
	}

	// URL format: /v1beta/models/{model}:generateContent?key={apiKey}
	model := g.cfg.Model
	if model == "" {
		model = "gemini-1.5-flash"
	}
	url := fmt.Sprintf(
		"%s/v1beta/models/%s:generateContent?key=%s",
		g.cfg.DefaultBaseURL(), model, g.cfg.APIKey,
	)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("gemini: create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := g.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("gemini: http: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("gemini: read body: %w", err)
	}

	var gr geminiResponse
	if err := json.Unmarshal(data, &gr); err != nil {
		return nil, fmt.Errorf("gemini: parse response: %w", err)
	}

	if gr.Error != nil {
		return nil, fmt.Errorf("gemini API error [%s]: %s", gr.Error.Status, gr.Error.Message)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gemini: unexpected status %d: %s", resp.StatusCode, string(data))
	}
	if len(gr.Candidates) == 0 || len(gr.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("gemini: empty response")
	}

	return ParseSuggestion(gr.Candidates[0].Content.Parts[0].Text), nil
}
