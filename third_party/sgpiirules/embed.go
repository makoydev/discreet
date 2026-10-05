// Package sgpiirules embeds a vendored, checksummed release of sg-pii-rules
// (https://github.com/makoydev/sg-pii-rules), the detection rules Discreet
// shares with Vetted. Never edit these files by hand: run
// scripts/vendor-sg-pii-rules.sh <tag>. A test checks them against SHA256SUMS.
package sgpiirules

import "embed"

// Files holds detectors.json, the schemas, the conformance fixtures,
// SHA256SUMS and VERSION ("<tag> <commit>").
//
//go:embed detectors.json SHA256SUMS VERSION schema/*.json fixtures/*.json
var Files embed.FS
