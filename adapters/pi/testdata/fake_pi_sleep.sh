#!/usr/bin/env bash
# fake pi that emits the session header then sleeps a long time, exercising
# context-cancel / abort teardown paths. Uses `exec` so bash replaces itself
# with sleep (single PID owning stdout); without exec, SIGKILL on bash would
# leave an orphaned sleep child still holding the pipe write-end and the
# parent's scanner would not observe EOF until the 30s elapsed.
echo '{"type":"session","version":3,"id":"sleeper","cwd":"."}'
exec sleep 30
