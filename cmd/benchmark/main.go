// Command benchmark measures Discreet's detection on synthetic text from an
// independent generator, and the latency the gateway adds. It prints
// Markdown for EVALS.md. Usage: go run ./cmd/benchmark [-n 5000] [-seed N]
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/makoydev/discreet/internal/audit"
	"github.com/makoydev/discreet/internal/bench"
	"github.com/makoydev/discreet/internal/detect"
	"github.com/makoydev/discreet/internal/gateway"
	"github.com/makoydev/discreet/internal/protect"
	"github.com/makoydev/discreet/internal/upstream"
	"github.com/makoydev/discreet/internal/vault"
	"github.com/makoydev/discreet/third_party/sgpiirules"
)

func main() {
	n := flag.Int("n", 5000, "samples")
	seed := flag.Uint64("seed", 20261005, "random seed")
	requests := flag.Int("requests", 2000, "requests per latency run")
	flag.Parse()

	engine, err := detect.Default()
	if err != nil {
		panic(err)
	}
	samples := bench.Generate(*n, *seed)
	version, _ := sgpiirules.Files.ReadFile("VERSION")
	fmt.Printf("Benchmark: %d samples, seed %d, rules sg-pii-rules %s, %s %s/%s\n\n", *n, *seed, strings.TrimSpace(string(version)), runtime.Version(), runtime.GOOS, runtime.GOARCH)

	results := bench.Evaluate(samples, engine)
	fmt.Println("| Entity | True values | Caught as the right kind (recall) | Protected by any detection | Detections | False alarms | Precision |")
	fmt.Println("| --- | --- | --- | --- | --- | --- | --- |")
	var keys []string
	for k := range results {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	tot := bench.Result{}
	for _, k := range keys {
		r := results[k]
		fmt.Printf("| `%s` | %d | %d (%.1f%%) | %d (%.1f%%) | %d | %d | %.1f%% |\n", k, r.Total, r.Caught, 100*r.Recall(), r.Protected, pct(r.Protected, r.Total), r.Detected, r.FalsePositives, 100*r.Precision())
		tot.Total += r.Total
		tot.Caught += r.Caught
		tot.Protected += r.Protected
		tot.Detected += r.Detected
		tot.FalsePositives += r.FalsePositives
	}
	fmt.Printf("| **All** | %d | %d (%.1f%%) | %d (%.1f%%) | %d | %d | %.1f%% |\n\n", tot.Total, tot.Caught, 100*tot.Recall(), tot.Protected, pct(tot.Protected, tot.Total), tot.Detected, tot.FalsePositives, 100*tot.Precision())

	fmt.Println("Recall by writing style:")
	fmt.Println()
	fmt.Println("| Entity | Style | Caught / total |")
	fmt.Println("| --- | --- | --- |")
	for _, k := range keys {
		var styles []string
		for v := range results[k].Variants {
			styles = append(styles, v)
		}
		sort.Strings(styles)
		for _, v := range styles {
			c := results[k].Variants[v]
			fmt.Printf("| `%s` | %s | %d / %d (%.0f%%) |\n", k, v, c[0], c[1], pct(c[0], c[1]))
		}
	}
	fmt.Println()
	fmt.Println("False-alarm examples (synthetic text):")
	for _, k := range keys {
		for _, ex := range results[k].FPExamples {
			fmt.Printf("- `%s`: %q\n", k, ex)
		}
	}

	fmt.Println()
	latency(samples, engine, *requests)
}

func pct(a, b int) float64 {
	if b == 0 {
		return 0
	}
	return 100 * float64(a) / float64(b)
}

// latency compares the full gateway (mock model, audit log on disk) with a
// baseline server that does the same JSON work and mock call without
// Discreet's processing. The difference is what Discreet adds.
func latency(samples []bench.Sample, engine *detect.Engine, requests int) {
	dir, _ := os.MkdirTemp("", "discreet-bench")
	defer os.RemoveAll(dir)
	log, err := audit.Open(filepath.Join(dir, "audit.jsonl"), []byte("benchmark-key-0123456789abcdefghijk"))
	if err != nil {
		panic(err)
	}
	defer log.Close()
	v, _ := vault.New(vault.DefaultTTL)
	gw := httptest.NewServer(gateway.New(gateway.Config{
		Engine: engine, Policy: protect.DefaultPolicy(), Vault: v, Audit: log,
		Logger: slog.New(slog.NewJSONHandler(io.Discard, nil)),
	}).Handler())
	defer gw.Close()
	mock := &upstream.Mock{}
	base := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Model    string             `json:"model"`
			Messages []upstream.Message `json:"messages"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		resp, _ := mock.Complete(context.Background(), upstream.Request{Model: req.Model, Messages: req.Messages})
		json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"role": "assistant", "content": resp.Content}}}})
	}))
	defer base.Close()

	run := func(url string) []time.Duration {
		var ds []time.Duration
		for i := 0; i < requests; i++ {
			body, _ := json.Marshal(map[string]any{"model": "mock", "messages": []map[string]string{{"role": "user", "content": samples[i%len(samples)].Text}}})
			req, _ := http.NewRequest("POST", url+"/v1/chat/completions", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Discreet-Purpose", "benchmark")
			start := time.Now()
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				panic(err)
			}
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			ds = append(ds, time.Since(start))
		}
		sort.Slice(ds, func(i, j int) bool { return ds[i] < ds[j] })
		return ds
	}
	run(gw.URL) // warm up
	run(base.URL)
	g, b := run(gw.URL), run(base.URL)
	q := func(ds []time.Duration, p float64) time.Duration { return ds[int(float64(len(ds)-1)*p)] }
	ms := func(d time.Duration) string { return fmt.Sprintf("%.2f ms", float64(d.Microseconds())/1000) }
	fmt.Printf("Latency over %d sequential requests on loopback, each with one benchmark sample:\n\n", requests)
	fmt.Println("| | p50 | p95 | p99 |")
	fmt.Println("| --- | --- | --- | --- |")
	fmt.Printf("| Baseline: same HTTP and JSON work, mock model, no Discreet | %s | %s | %s |\n", ms(q(b, .5)), ms(q(b, .95)), ms(q(b, .99)))
	fmt.Printf("| Through Discreet (detect, protect, restore, audit write with fsync) | %s | %s | %s |\n", ms(q(g, .5)), ms(q(g, .95)), ms(q(g, .99)))
	fmt.Printf("| **Added by Discreet** | **%s** | **%s** | %s |\n", ms(q(g, .5)-q(b, .5)), ms(q(g, .95)-q(b, .95)), ms(q(g, .99)-q(b, .99)))

	// Where the time goes: detection alone, and one durable audit write.
	var det, wr []time.Duration
	for i := 0; i < requests; i++ {
		start := time.Now()
		engine.Detect(samples[i%len(samples)].Text)
		det = append(det, time.Since(start))
		start = time.Now()
		log.Append(audit.Record{Decision: audit.Allowed})
		wr = append(wr, time.Since(start))
	}
	sort.Slice(det, func(i, j int) bool { return det[i] < det[j] })
	sort.Slice(wr, func(i, j int) bool { return wr[i] < wr[j] })
	us := func(d time.Duration) string { return fmt.Sprintf("%.3f ms", float64(d.Nanoseconds())/1e6) }
	fmt.Printf("| of which: detection alone | %s | %s | %s |\n", us(q(det, .5)), us(q(det, .95)), us(q(det, .99)))
	fmt.Printf("| of which: one audit write, flushed to disk | %s | %s | %s |\n", us(q(wr, .5)), us(q(wr, .95)), us(q(wr, .99)))
}
