package upstream

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// OpenAICompatible calls a /chat/completions endpoint: OpenAI itself, or a
// local Ollama (http://localhost:11434/v1). It sends only the protected
// messages and the few fields the gateway allows.
type OpenAICompatible struct {
	BaseURL         string // e.g. https://api.openai.com/v1
	APIKey          string // the provider's key; never the caller's token
	TokenParam      string // "max_completion_tokens" (OpenAI) or "max_tokens" (Ollama)
	ReasoningEffort string // optional, e.g. "low"; not sent when empty
	HTTP            *http.Client
}

func (*OpenAICompatible) Name() string { return "openai-compatible" }

// Complete sends one non-streaming chat completion.
func (o *OpenAICompatible) Complete(ctx context.Context, req Request) (Response, error) {
	body := map[string]any{"model": req.Model, "messages": req.Messages}
	if req.MaxTokens > 0 {
		param := o.TokenParam
		if param == "" {
			param = "max_completion_tokens"
		}
		body[param] = req.MaxTokens
	}
	if req.Temperature != nil {
		body["temperature"] = *req.Temperature
	}
	if o.ReasoningEffort != "" {
		body["reasoning_effort"] = o.ReasoningEffort
	}
	data, err := json.Marshal(body)
	if err != nil {
		return Response{}, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, "POST", strings.TrimRight(o.BaseURL, "/")+"/chat/completions", bytes.NewReader(data))
	if err != nil {
		return Response{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if o.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+o.APIKey)
	}
	client := o.HTTP
	if client == nil {
		client = &http.Client{Timeout: 90 * time.Second}
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		return Response{}, fmt.Errorf("upstream request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		// The provider's error body is not read or logged: only the status.
		return Response{}, fmt.Errorf("upstream returned HTTP %d", resp.StatusCode)
	}
	var out struct {
		Model   string `json:"model"`
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return Response{}, fmt.Errorf("upstream response is not valid JSON: %w", err)
	}
	if len(out.Choices) == 0 {
		return Response{}, errors.New("upstream response has no choices")
	}
	return Response{
		Content:      out.Choices[0].Message.Content,
		Model:        out.Model,
		FinishReason: out.Choices[0].FinishReason,
		PromptTokens: out.Usage.PromptTokens,
		OutputTokens: out.Usage.CompletionTokens,
	}, nil
}
