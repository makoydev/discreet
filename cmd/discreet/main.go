// Command discreet is a privacy gateway between applications and large
// language models. It replaces personal data with placeholders before a
// request reaches the model, puts the data back in the answer, and keeps a
// tamper-evident audit log.
//
// This is the skeleton from issue D1: each subcommand describes itself, and
// later issues build them out (serve in D5, audit in D7).
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/makoydev/discreet/internal/audit"
	"github.com/makoydev/discreet/internal/detect"
	"github.com/makoydev/discreet/internal/gateway"
	"github.com/makoydev/discreet/internal/protect"
	"github.com/makoydev/discreet/internal/upstream"
	"github.com/makoydev/discreet/internal/vault"
	"github.com/makoydev/discreet/third_party/sgpiirules"
)

// version is overridden at release time with -ldflags "-X main.version=...".
var version = "0.0.0-dev"

const usage = `discreet: a privacy gateway for large language model requests

Usage:
  discreet serve          run the gateway
  discreet audit verify   check that the audit log has not been changed
  discreet audit export   export the audit log as CSV
  discreet version        print the version
  discreet help           show this help

Run "discreet <command> --help" for details of a command.
`

const serveUsage = `Usage: discreet serve [-addr :8080] [-audit-log path] [-policy file.yaml]

Runs the gateway: an OpenAI-compatible POST /v1/chat/completions that swaps
personal data for placeholders before the request reaches the model, puts
it back in the answer, and records every request in the audit log.

Requires DISCREET_HMAC_KEY (at least 32 characters, e.g. openssl rand -hex 32).
Callers must send an X-Discreet-Purpose header.

By default every request goes to a free mock model. To reach a real model,
set DISCREET_UPSTREAM_URL and give callers access tokens; anyone without a
valid token still gets the mock.

  DISCREET_ADDR                   listen address (default :8080)
  DISCREET_AUDIT_LOG              audit log file (default discreet-audit.jsonl)
  DISCREET_POLICY                 policy YAML (default: built in)
  DISCREET_UPSTREAM_URL           e.g. https://api.openai.com/v1 or http://localhost:11434/v1
  DISCREET_UPSTREAM_API_KEY       the provider's key
  DISCREET_UPSTREAM_MODEL         default gpt-6-luna
  DISCREET_UPSTREAM_TOKEN_PARAM   max_completion_tokens (OpenAI) or max_tokens (Ollama)
  DISCREET_PRICE_INPUT_PER_M      US$ per million input tokens (known for gpt-6-luna)
  DISCREET_PRICE_OUTPUT_PER_M     US$ per million output tokens
  DISCREET_ACCESS_TOKENS          tenant:token,tenant2:token2 (tokens of 24+ characters)
  DISCREET_DAILY_BUDGET_USD       hard daily limit for the real model (default 0.20)
  DISCREET_MAX_OUTPUT_TOKENS      output cap per request (default 2048)
  DISCREET_REASONING_EFFORT       optional, e.g. low
  DISCREET_RATE_LIMIT_PER_MINUTE  per client address (default 30, bursts of 10)
  DISCREET_CLIENT_IP_HEADER       header set by a trusted proxy, e.g. Fly-Client-IP
`

const auditUsage = `Usage: discreet audit <verify|export> [-log path]

  verify        check every record's hash chain; fails, naming the record,
                if any byte was changed, a record deleted or two swapped
  export -csv   write the verified log as CSV to standard output

The log defaults to $DISCREET_AUDIT_LOG, or discreet-audit.jsonl.
`

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	serveContext = ctx
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// serveContext ends when the process is asked to stop; tests replace it.
var serveContext = context.Background()

// run executes one command and returns the process exit code:
// 0 for success, 1 for a command that failed, 2 for a usage error.
func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	switch args[0] {
	case "help", "-h", "--help":
		fmt.Fprint(stdout, usage)
		return 0
	case "version", "--version":
		fmt.Fprintln(stdout, "discreet", version)
		return 0
	case "serve":
		if wantsHelp(args[1:]) {
			fmt.Fprint(stdout, serveUsage)
			return 0
		}
		return runServe(serveContext, args[1:], stderr)
	case "audit":
		if wantsHelp(args[1:]) {
			fmt.Fprint(stdout, auditUsage)
			return 0
		}
		if len(args) < 2 {
			fmt.Fprint(stderr, auditUsage)
			return 2
		}
		switch args[1] {
		case "verify", "export":
			return runAudit(args[1], args[2:], stdout, stderr)
		}
		fmt.Fprintf(stderr, "discreet audit: unknown subcommand %q\n\n%s", args[1], auditUsage)
		return 2
	}
	fmt.Fprintf(stderr, "discreet: unknown command %q\n\n%s", args[0], usage)
	return 2
}

func wantsHelp(args []string) bool {
	return len(args) > 0 && (args[0] == "-h" || args[0] == "--help" || args[0] == "help")
}

func defaultLogPath() string {
	if p := os.Getenv("DISCREET_AUDIT_LOG"); p != "" {
		return p
	}
	return "discreet-audit.jsonl"
}

// runAudit verifies or exports the audit log. Neither needs the HMAC key:
// the chain is plain SHA-256, so anyone holding the file can check it.
func runAudit(sub string, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("discreet audit "+sub, flag.ContinueOnError)
	fs.SetOutput(stderr)
	path := fs.String("log", defaultLogPath(), "audit log file")
	fs.Bool("csv", true, "export as CSV (the only format)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	f, err := os.Open(*path)
	if err != nil {
		fmt.Fprintf(stderr, "discreet audit %s: %v\n", sub, err)
		return 1
	}
	defer f.Close()
	if sub == "export" {
		if err := audit.ExportCSV(f, stdout); err != nil {
			fmt.Fprintf(stderr, "discreet audit export: not exported, the log is broken: %v\n", err)
			return 1
		}
		return 0
	}
	s, err := audit.Verify(f)
	if err != nil {
		fmt.Fprintf(stderr, "FAILED: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "OK: %d records, chain intact.\nHead hash: %s\n", s.Records, s.HeadHash)
	return 0
}

func envOr(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}

// runServe starts the gateway and blocks until ctx ends.
func runServe(ctx context.Context, args []string, stderr io.Writer) int {
	fs := flag.NewFlagSet("discreet serve", flag.ContinueOnError)
	fs.SetOutput(stderr)
	addr := fs.String("addr", envOr("DISCREET_ADDR", ":8080"), "listen address")
	logPath := fs.String("audit-log", defaultLogPath(), "audit log file")
	policyPath := fs.String("policy", os.Getenv("DISCREET_POLICY"), "policy YAML file (default: built-in policy)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	fail := func(err error) int { fmt.Fprintf(stderr, "discreet serve: %v\n", err); return 1 }

	key, err := audit.KeyFromEnv()
	if err != nil {
		return fail(err)
	}
	policy := protect.DefaultPolicy()
	if *policyPath != "" {
		data, err := os.ReadFile(*policyPath)
		if err != nil {
			return fail(err)
		}
		if policy, err = protect.ParsePolicy(data); err != nil {
			return fail(err)
		}
	}
	engine, err := detect.Default()
	if err != nil {
		return fail(err)
	}
	v, err := vault.New(vault.DefaultTTL)
	if err != nil {
		return fail(err)
	}
	log, err := audit.Open(*logPath, key)
	if err != nil {
		return fail(err)
	}
	defer log.Close()

	logger := slog.New(slog.NewJSONHandler(stderr, nil))
	cfg := gateway.Config{Engine: engine, Policy: policy, Vault: v, Audit: log, Logger: logger}
	rate, err := envInt("DISCREET_RATE_LIMIT_PER_MINUTE", 30)
	if err != nil {
		return fail(err)
	}
	cfg.RateLimit = gateway.NewRateLimiter(rate, 10, os.Getenv("DISCREET_CLIENT_IP_HEADER"))
	upstreamInfo := "mock only"
	if url := os.Getenv("DISCREET_UPSTREAM_URL"); url != "" {
		if err := configureReal(&cfg, url, log.SpentToday); err != nil {
			return fail(err)
		}
		upstreamInfo = fmt.Sprintf("%s for %d access token(s), otherwise mock; budget US$%.2f/day; worst case US$%.4f per request",
			cfg.Model, cfg.Tokens.Len(), mustFloat("DISCREET_DAILY_BUDGET_USD", 0.20), cfg.Price.Ceiling(64<<10, 1, cfg.MaxOutputTokens))
		if cfg.Tokens.Len() == 0 {
			logger.Warn("DISCREET_UPSTREAM_URL is set but there are no DISCREET_ACCESS_TOKENS, so every request gets the mock")
		}
	}
	srv := &http.Server{
		Handler:           gateway.New(cfg).Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      120 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		return fail(err)
	}
	version, _ := sgpiirules.Files.ReadFile("VERSION")
	head, seq := log.Head()
	logger.Info("discreet listening", "addr", ln.Addr().String(), "upstream", upstreamInfo,
		"rules", "sg-pii-rules "+strings.TrimSpace(string(version)), "audit_log", *logPath, "audit_records", seq, "audit_head", head)

	go func() {
		t := time.NewTicker(time.Minute)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				v.Sweep()
			}
		}
	}()
	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(ln) }()
	select {
	case err := <-errc:
		if !errors.Is(err, http.ErrServerClosed) {
			return fail(err)
		}
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		srv.Shutdown(shutdown)
	}
	logger.Info("discreet stopped")
	return 0
}

// configureReal sets up the real model from the environment. Secrets are
// read here and never logged.
func configureReal(cfg *gateway.Config, url string, spentToday func() float64) error {
	tokens, err := gateway.ParseTokens(os.Getenv("DISCREET_ACCESS_TOKENS"))
	if err != nil {
		return err
	}
	model := envOr("DISCREET_UPSTREAM_MODEL", "gpt-6-luna")
	price, known := gateway.KnownPrices[model]
	in, inSet := os.LookupEnv("DISCREET_PRICE_INPUT_PER_M")
	out, outSet := os.LookupEnv("DISCREET_PRICE_OUTPUT_PER_M")
	if inSet || outSet {
		if price.Input, err = strconv.ParseFloat(in, 64); err != nil {
			return fmt.Errorf("DISCREET_PRICE_INPUT_PER_M: %w", err)
		}
		if price.Output, err = strconv.ParseFloat(out, 64); err != nil {
			return fmt.Errorf("DISCREET_PRICE_OUTPUT_PER_M: %w", err)
		}
	} else if !known {
		return fmt.Errorf("no known price for %s: set DISCREET_PRICE_INPUT_PER_M and DISCREET_PRICE_OUTPUT_PER_M (0 for a local model)", model)
	}
	budget, err := envFloat("DISCREET_DAILY_BUDGET_USD", 0.20)
	if err != nil {
		return err
	}
	maxOut, err := envInt("DISCREET_MAX_OUTPUT_TOKENS", 2048)
	if err != nil {
		return err
	}
	cfg.Real = &upstream.OpenAICompatible{
		BaseURL:         url,
		APIKey:          os.Getenv("DISCREET_UPSTREAM_API_KEY"),
		TokenParam:      envOr("DISCREET_UPSTREAM_TOKEN_PARAM", "max_completion_tokens"),
		ReasoningEffort: os.Getenv("DISCREET_REASONING_EFFORT"),
	}
	cfg.Tokens, cfg.Model, cfg.Price, cfg.MaxOutputTokens = tokens, model, price, maxOut
	cfg.Budget = gateway.NewBudget(budget, spentToday)
	return nil
}

func envInt(name string, fallback int) (int, error) {
	v := os.Getenv(name)
	if v == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("%s must be a positive whole number", name)
	}
	return n, nil
}

func envFloat(name string, fallback float64) (float64, error) {
	v := os.Getenv(name)
	if v == "" {
		return fallback, nil
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil || f < 0 {
		return 0, fmt.Errorf("%s must be a number of US dollars", name)
	}
	return f, nil
}

func mustFloat(name string, fallback float64) float64 {
	f, _ := envFloat(name, fallback)
	return f
}
