// Package upstream sends protected requests to a model. Every client sees
// only placeholder text; restoring values happens in the gateway.
package upstream

import (
	"context"
	"regexp"
	"strings"
)

// Message is one chat message, already protected.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Request is what the gateway forwards: only these fields, nothing else
// from the caller's request.
type Request struct {
	Model       string
	Messages    []Message
	MaxTokens   int
	Temperature *float64
}

// Response is the model's answer.
type Response struct {
	Content      string
	Model        string
	FinishReason string
	PromptTokens int
	OutputTokens int
}

// Client is a model provider.
type Client interface {
	Name() string
	Complete(ctx context.Context, req Request) (Response, error)
}

// Mock answers without calling any model, so it is free and works offline.
// It drafts a reply that repeats the placeholders it was given, which makes
// restoring them visible in the demo.
type Mock struct {
	// Last holds the most recent request, for tests that check what a
	// model would have seen.
	Last *Request
}

func (*Mock) Name() string { return "mock" }

var mockPlaceholder = regexp.MustCompile(`<[A-Z][A-Z_]*_\d+>`)

func (m *Mock) Complete(_ context.Context, req Request) (Response, error) {
	m.Last = &req
	var last string
	for _, msg := range req.Messages {
		if msg.Role == "user" {
			last = msg.Content
		}
	}
	reply := "This is Discreet's mock model: no AI was called. I only ever saw placeholders; Discreet put the real details back before you read this.\n\nYour message, as I received it:\n\n" + last
	if ps := mockPlaceholder.FindAllString(last, -1); len(ps) > 0 {
		reply += "\n\nDraft reply: Thanks. I've noted the details for " + strings.Join(unique(ps), ", ") + " and will follow up."
	}
	tokens := func(s string) int { return len(s)/4 + 1 }
	prompt := 0
	for _, msg := range req.Messages {
		prompt += tokens(msg.Content)
	}
	return Response{Content: reply, Model: "mock", FinishReason: "stop", PromptTokens: prompt, OutputTokens: tokens(reply)}, nil
}

func unique(items []string) []string {
	seen, out := map[string]bool{}, []string{}
	for _, s := range items {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
