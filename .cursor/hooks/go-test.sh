#!/bin/bash
# On agent stop, run go test if any .go file is dirty. Fail-open.
cat >/dev/null

changed=$(git diff --name-only HEAD -- '*.go' 2>/dev/null)
untracked=$(git ls-files --others --exclude-standard -- '*.go' 2>/dev/null)
if [ -z "$changed" ] && [ -z "$untracked" ]; then
	echo '{}'
	exit 0
fi

out=$(go test ./... 2>&1)
code=$?
printf '%s\n' "$out" >&2

if [ "$code" -eq 0 ]; then
	echo '{}'
	exit 0
fi

printf '%s' "$out" | python3 -c '
import json, sys
out = sys.stdin.read()
if len(out) > 4000:
    out = out[:4000] + "\n..."
print(json.dumps({"additional_context": "go test ./... failed:\n" + out}))
' 2>/dev/null || echo '{}'
exit 0
