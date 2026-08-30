#!/bin/bash
# Format a just-edited Go file. Always print valid JSON (fail-open).
input=$(cat)
path=$(printf '%s' "$input" | python3 -c '
import json, sys
data = json.load(sys.stdin)
path = data.get("file_path") or data.get("filePath") or data.get("path") or ""
if not path:
    f = data.get("file")
    if isinstance(f, str):
        path = f
    elif isinstance(f, dict):
        path = f.get("path") or f.get("file_path") or ""
print(path)
' 2>/dev/null) || path=""

if [ -n "$path" ] && [ "${path##*.}" = "go" ] && [ -f "$path" ]; then
	gofmt -w "$path" >/dev/null 2>&1 || true
fi

echo '{}'
