#!/usr/bin/env bash
# Copies a tagged sg-pii-rules release into third_party/sgpiirules and
# verifies every file against the release's SHA256SUMS (sg-pii-rules ADR 0004).
# Usage: scripts/vendor-sg-pii-rules.sh v0.2.0-rc.1
set -euo pipefail
tag=${1:?release tag, e.g. v0.2.0-rc.1}
dest=third_party/sgpiirules
base="https://raw.githubusercontent.com/makoydev/sg-pii-rules/$tag"

curl -fsSL "$base/SHA256SUMS" -o "$dest/SHA256SUMS"
while read -r _ path; do
  mkdir -p "$dest/$(dirname "$path")"
  curl -fsSL "$base/$path" -o "$dest/$path"
done < "$dest/SHA256SUMS"
(cd "$dest" && if command -v sha256sum >/dev/null; then sha256sum -c SHA256SUMS; else shasum -a 256 -c SHA256SUMS; fi)

commit=$(git ls-remote https://github.com/makoydev/sg-pii-rules.git "refs/tags/$tag^{}" "refs/tags/$tag" | head -1 | cut -c1-7)
echo "$tag $commit" > "$dest/VERSION"
echo "vendored sg-pii-rules $tag ($commit)"
