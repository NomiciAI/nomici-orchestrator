#!/usr/bin/env bash
set -euo pipefail

interval="${1:-5}"
log_path="${FD_WATCH_LOG:-$HOME/Desktop/nomici-fd-watch.log}"

echo "Writing fd watch log to $log_path" >&2
while true; do
  {
    printf '\n%s\n' "$(date)"
    sysctl kern.num_files kern.maxfiles 2>/dev/null || true
    ps -axo pid,ppid,command | grep -E 'nomici|hermes|openclaw|node|python' | grep -v grep || true
    for pid in $(pgrep -f 'nomici|hermes|openclaw|node|python' 2>/dev/null | sort -u); do
      command_name="$(ps -p "$pid" -o comm= 2>/dev/null || true)"
      fd_count="$(lsof -nP -p "$pid" 2>/dev/null | awk 'END {print NR > 0 ? NR - 1 : 0}')"
      printf '%s\t%s\t%s\n' "$fd_count" "$pid" "$command_name"
    done | sort -nr | head -40
  } | tee -a "$log_path"
  sleep "$interval"
done
