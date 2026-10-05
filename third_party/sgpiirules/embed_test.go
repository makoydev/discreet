package sgpiirules

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"strings"
	"testing"
)

// The vendored files must be exactly the release's files (sg-pii-rules ADR
// 0004): a hand edit, or a file added without updating SHA256SUMS, fails here.
func TestFilesMatchSHA256SUMS(t *testing.T) {
	sums, err := Files.ReadFile("SHA256SUMS")
	if err != nil {
		t.Fatal(err)
	}
	listed := map[string]bool{}
	for _, line := range strings.Split(strings.TrimSpace(string(sums)), "\n") {
		want, path, ok := strings.Cut(line, "  ")
		if !ok {
			t.Fatalf("malformed SHA256SUMS line %q", line)
		}
		data, err := Files.ReadFile(path)
		if err != nil {
			t.Errorf("%s: listed in SHA256SUMS but not vendored: %v", path, err)
			continue
		}
		got := sha256.Sum256(data)
		if hex.EncodeToString(got[:]) != want {
			t.Errorf("%s: checksum does not match SHA256SUMS; re-run scripts/vendor-sg-pii-rules.sh", path)
		}
		listed[path] = true
	}
	err = fs.WalkDir(Files, ".", func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && path != "SHA256SUMS" && path != "VERSION" && !listed[path] {
			t.Errorf("%s: vendored but not listed in SHA256SUMS", path)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestVersionMatchesDetectors(t *testing.T) {
	version, _ := Files.ReadFile("VERSION")
	tag, _, _ := strings.Cut(strings.TrimSpace(string(version)), " ")
	var rules struct{ Version string }
	data, _ := Files.ReadFile("detectors.json")
	if err := json.Unmarshal(data, &rules); err != nil {
		t.Fatal(err)
	}
	release, _, _ := strings.Cut(strings.TrimPrefix(tag, "v"), "-")
	if release != rules.Version {
		t.Errorf("VERSION says %s but detectors.json says %s", tag, rules.Version)
	}
}
