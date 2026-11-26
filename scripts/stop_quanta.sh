#!/usr/bin/env bash
set -euo pipefail

# Cleanly stops native QUANTA services tracked in .logs/*.pid and/or listening on ports 8000, 8001, 8080, 3000.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
LOG_DIR="$REPO_ROOT/.logs"

stopped=0

if [[ -d "$LOG_DIR" ]]; then
  for pid_file in "$LOG_DIR"/*.pid; do
    [[ -e "$pid_file" ]] || continue
    pid="$(tr -d '[:space:]' < "$pid_file")"
    if [[ "$pid" =~ ^[0-9]+$ ]] && kill -0 "$pid" 2>/dev/null; then
      kill "$pid" 2>/dev/null || true
      sleep 0.5
      if kill -0 "$pid" 2>/dev/null; then
        kill -9 "$pid" 2>/dev/null || true
      fi
      printf "[OK]     Stopped %s (PID %s)\n" "$(basename "$pid_file" .pid)" "$pid"
      stopped=$((stopped + 1))
    fi
    rm -f "$pid_file"
  done
fi

for port in 8000 8001 8080 3000; do
  if command -v lsof >/dev/null 2>&1; then
    pids="$(lsof -tiTCP:"$port" -sTCP:LISTEN 2>/dev/null || true)"
    for p in $pids; do
      if kill -0 "$p" 2>/dev/null; then
        kill "$p" 2>/dev/null || kill -9 "$p" 2>/dev/null || true
        printf "[OK]     Stopped process %s on port %s\n" "$p" "$port"
        stopped=$((stopped + 1))
      fi
    done
  elif command -v fuser >/dev/null 2>&1; then
    fuser -k "${port}/tcp" >/dev/null 2>&1 && stopped=$((stopped + 1)) || true
  fi
done

if (( stopped == 0 )); then
  printf "[QUANTA] No active processes found.\n"
else
  printf "[QUANTA] Stopped %d service process(es).\n" "$stopped"
fi
