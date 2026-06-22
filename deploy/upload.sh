#!/usr/bin/env bash
set -Eeuo pipefail

SOURCE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROLE="${1:-master}"

MASTER_REMOTE_HOST="${MASTER_REMOTE_HOST:-154.51.41.73}"
WORKER_REMOTE_HOST="${WORKER_REMOTE_HOST:-154.51.41.215}"
REMOTE_HOST="${REMOTE_HOST:-}"
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

remote_host_for_role() {
  case "$ROLE" in
    master)
      echo "${REMOTE_HOST:-$MASTER_REMOTE_HOST}"
      ;;
    worker|work|slave)
      echo "${REMOTE_HOST:-$WORKER_REMOTE_HOST}"
      ;;
    *)
      echo "Usage: $0 [master|worker]" >&2
      exit 1
      ;;
  esac
}

role_dirs() {
  case "$ROLE" in
    master)
      printf '%s\n' master nginx
      ;;
    worker|work|slave)
      printf '%s\n' work
      ;;
  esac
}

upload_file_if_exists() {
  local source_path="$1"
  local target_path="$2"
  if [[ -f "$source_path" ]]; then
    scp "${SCP_OPTS[@]}" "$source_path" "$REMOTE_TARGET:$target_path"
  fi
}

upload_dir_if_exists() {
  local source_path="$1"
  if [[ -d "$source_path" ]]; then
    scp "${SCP_OPTS[@]}" -r "$source_path" "$REMOTE_TARGET:$REMOTE_DIR/"
  fi
}

require_command ssh
require_command scp

TARGET_HOST="$(remote_host_for_role)"
REMOTE_TARGET="${REMOTE_USER}@${TARGET_HOST}"
REMOTE_DIR_QUOTED="$(printf "%q" "$REMOTE_DIR")"

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

REMOTE_SUBDIRS="$REMOTE_DIR_QUOTED"
while IFS= read -r dir; do
  REMOTE_SUBDIRS+=" $REMOTE_DIR_QUOTED/$dir"
done < <(role_dirs)

ssh "${SSH_OPTS[@]}" "$REMOTE_TARGET" "mkdir -p $REMOTE_SUBDIRS"

scp "${SCP_OPTS[@]}" "$SOURCE_DIR/deploy.sh" "$REMOTE_TARGET:$REMOTE_DIR/"
scp "${SCP_OPTS[@]}" "$SOURCE_DIR/upload.sh" "$REMOTE_TARGET:$REMOTE_DIR/"
scp "${SCP_OPTS[@]}" "$SOURCE_DIR/up.sh" "$REMOTE_TARGET:$REMOTE_DIR/"

while IFS= read -r dir; do
  upload_dir_if_exists "$SOURCE_DIR/$dir"
done < <(role_dirs)

upload_file_if_exists "$SOURCE_DIR/artifacts/new-api-latest.tar" "$REMOTE_DIR/new-api-latest.tar"
upload_file_if_exists "$SOURCE_DIR/new-api-latest.tar" "$REMOTE_DIR/new-api-latest.tar"

echo "Upload completed for $ROLE: $REMOTE_TARGET:$REMOTE_DIR"
