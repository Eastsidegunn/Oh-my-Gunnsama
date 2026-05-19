#!/usr/bin/env bash
# Real-pi E2E smoke for `omg run`. Requires ANTHROPIC_API_KEY and an
# installed pi-coding-agent on $PATH (or via workers.pi.binary_path).
# Manually gated: owner runs before tagging v0.1.0. Not for CI.
set -euo pipefail

: "${ANTHROPIC_API_KEY:?ANTHROPIC_API_KEY is required for real-pi smoke}"

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO="${REPO:-$(cd "$SCRIPT_DIR/.." && pwd)}"
OMG="${OMG:-$REPO/omg}"

if [[ ! -x "$OMG" ]]; then
  echo "FAIL: omg binary not found at $OMG (build with: go build -o omg ./cmd/omg)"
  exit 1
fi

if ! command -v pi >/dev/null 2>&1 && [[ -z "${PI_BINARY_PATH:-}" ]]; then
  echo "FAIL: pi not on PATH and PI_BINARY_PATH unset"
  exit 1
fi

TMPDIR="$(mktemp -d)"
trap 'rm -rf "$TMPDIR"' EXIT
cd "$TMPDIR"

mkdir -p .omg
cat > .omg/config.yaml <<EOF
workers:
  pi:
    kind: pi
$( [[ -n "${PI_BINARY_PATH:-}" ]] && echo "    binary_path: $PI_BINARY_PATH" )
storage:
  database:
    path: $TMPDIR/.omg/omg.db
EOF

"$OMG" run "create a file named hello.txt with the single word: hi"

test -f hello.txt || { echo "FAIL: hello.txt not created"; exit 1; }
CONTENT="$(cat hello.txt | tr -d '[:space:]')"
[[ "$CONTENT" == "hi" ]] || { echo "FAIL: content = $(cat hello.txt) (want 'hi')"; exit 1; }

RUNS=$(sqlite3 .omg/omg.db "SELECT COUNT(*) FROM runs")
AGENTS=$(sqlite3 .omg/omg.db "SELECT COUNT(*) FROM agent_sessions WHERE backend_kind='pi'")
EVENTS=$(sqlite3 .omg/omg.db "SELECT COUNT(*) FROM events")

[[ "$RUNS" -ge 1 ]] || { echo "FAIL: runs=$RUNS"; exit 1; }
[[ "$AGENTS" -ge 1 ]] || { echo "FAIL: agent_sessions=$AGENTS"; exit 1; }
[[ "$EVENTS" -ge 5 ]] || { echo "FAIL: events=$EVENTS"; exit 1; }

echo "PASS: hello run (runs=$RUNS agent_sessions=$AGENTS events=$EVENTS)"
