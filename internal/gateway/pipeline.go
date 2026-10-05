package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/makoydev/discreet/internal/audit"
	"github.com/makoydev/discreet/internal/protect"
	"github.com/makoydev/discreet/internal/upstream"
)

// call is one request, already parsed, from the API or the demo page.
type call struct {
	id, tenant, purpose, model string
	roles, texts               []string
	maxTokens                  int
	temperature                *float64
	up                         upstream.Client
	real                       bool
}

// outcome is everything a request produced. status is 200 for an answer;
// otherwise code and message explain the refusal.
type outcome struct {
	status        int
	code, message string
	record        audit.Record
	sent          []upstream.Message // what the model received
	response      upstream.Response  // what the model answered, placeholders and all
	parts         []protect.Part     // the restored answer, split for display
	answer        string             // the restored answer with the footer
}

// process is the whole pipeline, shared by the API and the demo page so
// the demo shows exactly what the API does: purpose check, protection,
// budget, model call, restore, footer and audit record.
func (s *Server) process(ctx context.Context, c call) outcome {
	start := time.Now()
	log := s.cfg.Logger.With("request_id", c.id)
	refuse := func(rec audit.Record, status int, code, message string) outcome {
		saved, ok := s.finish(log, rec, start)
		if !ok {
			return outcome{status: http.StatusInternalServerError, code: "audit_failed", message: "the request could not be recorded, so it was not processed"}
		}
		return outcome{status: status, code: code, message: message, record: saved}
	}

	// The purpose is required, recorded with any personal data removed,
	// and refused outright for decisions about people's eligibility.
	if c.purpose == "" {
		return outcome{status: http.StatusBadRequest, code: "missing_purpose", message: "set the X-Discreet-Purpose header to say what this request is for, for example \"claims summary\""}
	}
	model := c.model
	if c.real {
		if c.model != "" && c.model != s.cfg.Model {
			return outcome{status: http.StatusBadRequest, code: "model_not_allowed", message: fmt.Sprintf("this gateway only sends requests to %s", s.cfg.Model)}
		}
		model = s.cfg.Model
	}
	rec := audit.Record{RequestID: c.id, Tenant: c.tenant, Purpose: s.scrub(c.purpose), Upstream: c.up.Name(), Model: model}
	if deniedPurpose(c.purpose, s.cfg.Policy.DeniedPurposes) {
		rec.Decision, rec.Reason = audit.RefusedPurpose, "purpose refused by policy"
		return refuse(rec, http.StatusForbidden, "purpose_refused", "Discreet does not take part in automated decisions about people's eligibility. A person must make that decision.")
	}

	// Protect every message in one session, so placeholders are consistent.
	session := protect.NewSession(s.cfg.Policy, s.cfg.Vault)
	defer session.Close()
	msgs := make([]upstream.Message, len(c.texts))
	for i, t := range c.texts {
		protected, err := session.Protect(t, s.cfg.Engine.Detect(t))
		var blocked *protect.BlockedError
		if errors.As(err, &blocked) {
			rec.Decision, rec.Reason, rec.Entities = audit.BlockedEntity, blocked.Error(), session.Counts()
			return refuse(rec, http.StatusForbidden, "entity_blocked", blocked.Error())
		}
		msgs[i] = upstream.Message{Role: c.roles[i], Content: protected}
	}
	rec.Entities = session.Counts()
	promptJSON, _ := json.Marshal(c.texts)
	rec.PromptHMAC = s.cfg.Audit.HMAC(string(promptJSON))

	maxTokens := c.maxTokens
	if maxTokens <= 0 || maxTokens > s.cfg.MaxOutputTokens {
		maxTokens = s.cfg.MaxOutputTokens
	}
	// Real calls reserve their worst-case cost first (ADR 0007).
	if c.real {
		promptBytes := 0
		for _, m := range msgs {
			promptBytes += len(m.Content)
		}
		ceiling := s.cfg.Price.Ceiling(promptBytes, len(msgs), maxTokens)
		if !s.cfg.Budget.Reserve(ceiling) {
			rec.Decision, rec.Reason = audit.RefusedBudget, fmt.Sprintf("daily budget reached (this request could cost up to US$%.4f)", ceiling)
			return refuse(rec, http.StatusTooManyRequests, "budget_exceeded", "today's spending limit for the AI model has been reached; requests resume tomorrow (UTC)")
		}
		defer s.cfg.Budget.Release(ceiling) // after the real cost is recorded below
	}

	resp, err := c.up.Complete(ctx, upstream.Request{Model: model, Messages: msgs, MaxTokens: maxTokens, Temperature: c.temperature})
	if err != nil {
		log.Warn("upstream error", "error", err.Error())
		rec.Decision, rec.Reason = audit.UpstreamError, "the model provider returned an error"
		return refuse(rec, http.StatusBadGateway, "upstream_error", "the model provider returned an error")
	}

	parts := session.RestoreParts(resp.Content)
	answer := protect.Join(parts) + fmt.Sprintf(Footer, c.id)
	rec.Decision, rec.Model, rec.ResponseHMAC = audit.Allowed, resp.Model, s.cfg.Audit.HMAC(answer)
	if c.real {
		rec.CostUSD = s.cfg.Price.Cost(resp.PromptTokens, resp.OutputTokens)
	}
	saved, ok := s.finish(log, rec, start)
	if !ok {
		return outcome{status: http.StatusInternalServerError, code: "audit_failed", message: "the request could not be recorded, so it was not processed"}
	}
	return outcome{status: http.StatusOK, record: saved, sent: msgs, response: resp, parts: parts, answer: answer}
}
