// Command discreet is a privacy gateway between applications and large
// language models. It replaces personal data with placeholders before a
// request reaches the model, puts the data back in the answer, and keeps a
// tamper-evident audit log.
//
// This is the skeleton from issue D1: each subcommand describes itself, and
// later issues build them out (serve in D5, audit in D7).
package main

import (
	"fmt"
	"io"
	"os"
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

const auditUsage = `Usage: discreet audit <verify|export>

  verify   check every record's hash chain; fails if any byte was changed
  export   write the audit log as CSV

Not built yet (issue D7).
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
			fmt.Fprintf(stderr, "discreet audit %s: not built yet (issue D7)\n", args[1])
			return 1
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
