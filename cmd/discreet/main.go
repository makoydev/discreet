// Command discreet is a privacy gateway between applications and large
// language models. It replaces personal data with placeholders before a
// request reaches the model, puts the data back in the answer, and keeps a
// tamper-evident audit log.
//
// This is the skeleton from issue D1: each subcommand describes itself, and
// later issues build them out (serve in D5, audit in D7).
package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/makoydev/discreet/internal/audit"
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

const serveUsage = `Usage: discreet serve

Runs the gateway: an OpenAI-compatible endpoint that swaps personal data for
placeholders before the request reaches the model. Not built yet (issue D5).
`

const auditUsage = `Usage: discreet audit <verify|export> [-log path]

  verify        check every record's hash chain; fails, naming the record,
                if any byte was changed, a record deleted or two swapped
  export -csv   write the verified log as CSV to standard output

The log defaults to $DISCREET_AUDIT_LOG, or discreet-audit.jsonl.
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

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
		fmt.Fprintln(stderr, "discreet serve: not built yet (issue D5)")
		return 1
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
