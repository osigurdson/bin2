#!/usr/bin/env bash

set -euo pipefail

SESSION="bin2-local"
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

if ! command -v tmux >/dev/null 2>&1; then
  echo "tmux is required but not installed."
  exit 1
fi

if tmux has-session -t "$SESSION" 2>/dev/null; then
  tmux attach -t "$SESSION"
  exit 0
fi

tmux new-session -d -s "$SESSION" -n dev
tmux split-window -h -t "$SESSION:dev"

tmux send-keys -t "$SESSION:dev.0" \
  "cd \"$ROOT_DIR\" && set -a && source .env && source ui/.env.local && set +a && DEBUG=1 go run ./cmd/api" C-m

tmux send-keys -t "$SESSION:dev.1" \
  "cd \"$ROOT_DIR/ui\" && NEXT_PUBLIC_API_BASE_URL=http://localhost:5000 npm run dev" C-m

tmux select-pane -t "$SESSION:dev.1"
tmux attach -t "$SESSION"
