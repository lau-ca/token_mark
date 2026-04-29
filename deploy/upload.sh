#!/usr/bin/env bash
set -Eeuo pipefail

SOURCE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

REMOTE_HOST="${REMOTE_HOST:-43.133.200.73}"
REMOTE_USER="${REMOTE_USER:-root}"
REMOTE_DIR="${REMOTE_DIR:-/root/gateway}"
SSH_PORT="${SSH_PORT:-22}"
SSH_KEY="${SSH_KEY:-}"

require_command() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "ERROR: required command not found: $1" >&2
    exit 1
  fi
}

require_command ssh
require_command scp

SSH_OPTS=(
  -p "$SSH_PORT"
  -o ServerAliveInterval=30
  -o ServerAliveCountMax=3
)

SCP_OPTS=(
  -P "$SSH_PORT"
  -o ServerAliveInterval=30
  -o ServerAliveCountMax=3
)

if [[ -n "$SSH_KEY" ]]; then
  SSH_OPTS+=(-i "$SSH_KEY")
  SCP_OPTS+=(-i "$SSH_KEY")
fi

REMOTE_TARGET="${REMOTE_USER}@${REMOTE_HOST}"
REMOTE_DIR_QUOTED="$(printf "%q" "$REMOTE_DIR")"

echo "[1/3] Source: $SOURCE_DIR"
echo "[2/3] Target: $REMOTE_TARGET:$REMOTE_DIR"
echo "[3/3] Uploading deploy files..."

ssh "${SSH_OPTS[@]}" "$REMOTE_TARGET" \
  "mkdir -p $REMOTE_DIR_QUOTED $REMOTE_DIR_QUOTED/master $REMOTE_DIR_QUOTED/work $REMOTE_DIR_QUOTED/postgres $REMOTE_DIR_QUOTED/nginx $REMOTE_DIR_QUOTED/artifacts"

scp "${SCP_OPTS[@]}" "$SOURCE_DIR/deploy.sh" "$REMOTE_TARGET:$REMOTE_DIR/"
scp "${SCP_OPTS[@]}" "$SOURCE_DIR/upload.sh" "$REMOTE_TARGET:$REMOTE_DIR/"

scp "${SCP_OPTS[@]}" -r "$SOURCE_DIR/master" "$REMOTE_TARGET:$REMOTE_DIR/"
scp "${SCP_OPTS[@]}" -r "$SOURCE_DIR/work" "$REMOTE_TARGET:$REMOTE_DIR/"
scp "${SCP_OPTS[@]}" -r "$SOURCE_DIR/postgres" "$REMOTE_TARGET:$REMOTE_DIR/"
scp "${SCP_OPTS[@]}" -r "$SOURCE_DIR/nginx" "$REMOTE_TARGET:$REMOTE_DIR/"

if [[ -d "$SOURCE_DIR/artifacts" ]]; then
  scp "${SCP_OPTS[@]}" -r "$SOURCE_DIR/artifacts" "$REMOTE_TARGET:$REMOTE_DIR/"
fi

if [[ -f "$SOURCE_DIR/new-api-latest.tar" ]]; then
  scp "${SCP_OPTS[@]}" "$SOURCE_DIR/new-api-latest.tar" "$REMOTE_TARGET:$REMOTE_DIR/"
fi

echo "Upload completed."
