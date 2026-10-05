# One command for every check, used locally and in CI: make check
GOVULNCHECK := golang.org/x/vuln/cmd/govulncheck@v1.8.0

.PHONY: check fmt vet test vuln build bench

check: fmt vet test vuln build

fmt:
	@unformatted=$$(gofmt -l .); if [ -n "$$unformatted" ]; then echo "gofmt needed:"; echo "$$unformatted"; exit 1; fi

vet:
	go vet ./...

test:
	go test -race -count=1 ./...

vuln:
	go run $(GOVULNCHECK) ./...

build:
	go build -o bin/discreet ./cmd/discreet

# Detection benchmark and latency, printed as Markdown for EVALS.md
bench:
	go run ./cmd/benchmark
