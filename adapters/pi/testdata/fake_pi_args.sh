#!/usr/bin/env bash
printf '%s\n' "$*" > "$FAKE_PI_ARGS_PATH"
cat <<'EOF'
{"type":"session","version":3,"id":"fake-session","timestamp":"2026-05-19T10:00:00.000Z","cwd":"."}
{"type":"agent_start","timestamp":"2026-05-19T10:00:00.100Z"}
{"type":"agent_end","timestamp":"2026-05-19T10:00:02.100Z","messages":[]}
EOF
exit 0
