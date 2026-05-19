#!/usr/bin/env bash
# Fake pi --mode json substitute used by qa_hello_run_fake.sh.
# Emits canned JSONL events on stdout AND actually writes hello.txt
# in the current working directory so the E2E QA can observe both
# the file side-effect and the SQLite event ledger without a real LLM.

echo "hi" > hello.txt

cat <<'EOF'
{"type":"session","version":3,"id":"fake-session","timestamp":"2026-05-19T10:00:00.000Z","cwd":"."}
{"type":"agent_start","timestamp":"2026-05-19T10:00:00.100Z"}
{"type":"tool_execution_start","timestamp":"2026-05-19T10:00:01.000Z","toolCallId":"tool-1","toolName":"write","args":{"path":"hello.txt","content":"hi"}}
{"type":"tool_execution_end","timestamp":"2026-05-19T10:00:01.500Z","toolCallId":"tool-1","toolName":"write","result":{"path":"hello.txt","bytes":2}}
{"type":"message_end","timestamp":"2026-05-19T10:00:02.000Z","message":{"role":"assistant","content":"Done."}}
{"type":"agent_end","timestamp":"2026-05-19T10:00:02.100Z","messages":[]}
EOF
exit 0
