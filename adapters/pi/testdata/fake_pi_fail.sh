#!/usr/bin/env bash
# fake pi failure: stderr-only diagnostic + non-zero exit.
echo "fake pi: simulated failure" >&2
exit 1
