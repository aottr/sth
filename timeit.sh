#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 1 ]]; then
  echo "Usage: $0 <command> [args...]" >&2
  exit 1
fi

start_ns=$(date +%s%N)
"$@"
status=$?
end_ns=$(date +%s%N)

elapsed_ns=$((end_ns - start_ns))
printf "elapsed: %.3f s (%.0f ms)\n" \
  "$(bc -l <<< "$elapsed_ns/1000000000")" \
  "$(bc -l <<< "$elapsed_ns/1000000")"

exit $status
