#!/bin/sh
set -eu
# Run the maintained unified backend + frontend lifecycle; Ctrl-C stops both.
task_repo=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$task_repo"
exec make dev "$@"
