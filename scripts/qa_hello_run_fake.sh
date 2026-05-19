#!/usr/bin/env bash
# CI-safe E2E smoke for `omg run`. Uses scripts/fake_pi_writes_hello.sh as
# the worker binary, bypassing any need for ANTHROPIC_API_KEY or a real pi
# install. Verifies the full pipeline: CLI dispatch -> inline fallback ->
# orchestrator.handleRun -> Worker spawn -> stdout JSONL parse -> SQLite
# event recording + actual hello.txt creation by the fake worker.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO="${REPO:-$(cd "$SCRIPT_DIR/.." && pwd)}"
OMG="${OMG:-$REPO/omg}"

if [[ ! -x "$OMG" ]]; then
  echo "FAIL: omg binary not found at $OMG (build with: go build -o omg ./cmd/omg)"
  exit 1
fi

FAKE_PI="$REPO/scripts/fake_pi_writes_hello.sh"
if [[ ! -x "$FAKE_PI" ]]; then
  echo "FAIL: fake pi not executable at $FAKE_PI"
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
    binary_path: $FAKE_PI
storage:
  database:
    path: $TMPDIR/.omg/omg.db
EOF

"$OMG" run "create hello.txt with content 'hi'" > /tmp/qa_omg_out.json 2> /tmp/qa_omg_err.log || {
  echo "omg run exit $?"
  echo "--- stdout ---"
  cat /tmp/qa_omg_out.json
  echo "--- stderr ---"
  cat /tmp/qa_omg_err.log
  exit 1
}

if [[ ! -f hello.txt ]]; then
  echo "FAIL: hello.txt not created in $TMPDIR"
  ls -la
  exit 1
fi

CONTENT="$(cat hello.txt)"
if [[ "$CONTENT" != "hi" ]]; then
  echo "FAIL: hello.txt content = $CONTENT (want 'hi')"
  exit 1
fi

RUNS=$(sqlite3 .omg/omg.db "SELECT COUNT(*) FROM runs")
AGENTS=$(sqlite3 .omg/omg.db "SELECT COUNT(*) FROM agent_sessions WHERE backend_kind='pi'")
EVENTS=$(sqlite3 .omg/omg.db "SELECT COUNT(*) FROM events")
TOOLS=$(sqlite3 .omg/omg.db "SELECT COUNT(*) FROM tool_calls")
WORK_ITEMS=$(sqlite3 .omg/omg.db "SELECT COUNT(*) FROM work_items")
WORK_COMPLETED=$(sqlite3 .omg/omg.db "SELECT COUNT(*) FROM work_items WHERE status='completed'")
TOOL_WORK_FK=$(sqlite3 .omg/omg.db "SELECT COUNT(*) FROM tool_calls WHERE work_id IS NOT NULL")

if [[ "$RUNS" -lt 1 ]]; then echo "FAIL: runs=$RUNS (want >=1)"; exit 1; fi
if [[ "$AGENTS" -lt 1 ]]; then echo "FAIL: agent_sessions(backend_kind=pi)=$AGENTS (want >=1)"; exit 1; fi
if [[ "$EVENTS" -lt 5 ]]; then echo "FAIL: events=$EVENTS (want >=5)"; exit 1; fi
if [[ "$TOOLS" -lt 1 ]]; then echo "FAIL: tool_calls=$TOOLS (want >=1)"; exit 1; fi
if [[ "$WORK_ITEMS" -lt 1 ]]; then echo "FAIL: work_items=$WORK_ITEMS (want >=1)"; exit 1; fi
if [[ "$WORK_COMPLETED" -lt 1 ]]; then echo "FAIL: work_items completed=$WORK_COMPLETED (want >=1)"; exit 1; fi
if [[ "$TOOL_WORK_FK" -lt 1 ]]; then echo "FAIL: tool_calls with work_id=$TOOL_WORK_FK (want >=1; check WorkID propagation)"; exit 1; fi

echo "PASS: hello run (runs=$RUNS agent_sessions=$AGENTS events=$EVENTS tool_calls=$TOOLS work_items=$WORK_ITEMS work_completed=$WORK_COMPLETED tool_work_fk=$TOOL_WORK_FK)"
