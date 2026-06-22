#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat >&2 <<'USAGE'
Usage:
  ./migrate-to-server.sh [target-host] [target-user]

Environment:
  SSH_PORT=22                   SSH port
  SSH_KEY=                      Optional SSH private key path

Example:
  ./migrate-to-server.sh 203.0.113.10
  ./migrate-to-server.sh 203.0.113.10 root
USAGE
}

require_command() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "ERROR: required command not found: $1" >&2
    exit 1
  fi
}

remote_quote() {
  printf "%q" "$1"
}

rsync_path() {
  local source_path="$1"
  local target_path="$2"

  if [ ! -e "$source_path" ]; then
    return 0
  fi

  ssh "${SSH_OPTS[@]}" "$REMOTE_TARGET" "mkdir -p $(remote_quote "$(dirname "$target_path")")"
  rsync "${RSYNC_OPTS[@]}" -e "$RSYNC_SSH" "$source_path" "$REMOTE_TARGET:$target_path"
}

collect_bind_sources() {
  local container_ids
  container_ids="$(docker ps -aq)"
  if [ -z "$container_ids" ]; then
    return 0
  fi

  # shellcheck disable=SC2086
  docker inspect $container_ids \
    --format '{{range .Mounts}}{{if eq .Type "bind"}}{{.Source}}{{"\n"}}{{end}}{{end}}' 2>/dev/null \
    | sed '/^$/d' \
    | sort -u
}

collect_volume_names() {
  local container_ids
  container_ids="$(docker ps -aq)"
  if [ -z "$container_ids" ]; then
    return 0
  fi

  # shellcheck disable=SC2086
  docker inspect $container_ids \
    --format '{{range .Mounts}}{{if eq .Type "volume"}}{{.Name}}{{"\n"}}{{end}}{{end}}' 2>/dev/null \
    | sed '/^$/d' \
    | sort -u
}

TARGET_HOST="${1:-}"
TARGET_USER="${2:-}"
GATEWAY_DIR="/root/gateway"
TARGET_DIR="/root/gateway"

if [ -z "$TARGET_HOST" ]; then
  printf "Target server IP: "
  read -r TARGET_HOST
fi

if [ -z "$TARGET_USER" ]; then
  printf "Target SSH user [root]: "
  read -r TARGET_USER
  TARGET_USER="${TARGET_USER:-root}"
fi

if [ -z "$TARGET_HOST" ]; then
  echo "ERROR: target server IP is required" >&2
  exit 1
fi

if hostname -I 2>/dev/null | tr ' ' '\n' | grep -Fx "$TARGET_HOST" >/dev/null 2>&1; then
  echo "ERROR: target host is this source server: $TARGET_HOST" >&2
  exit 1
fi

SSH_PORT="${SSH_PORT:-22}"
SSH_KEY="${SSH_KEY:-}"

if [ ! -d "$GATEWAY_DIR" ]; then
  echo "ERROR: gateway directory not found: $GATEWAY_DIR" >&2
  exit 1
fi

require_command docker
require_command rsync
require_command ssh
require_command scp

REMOTE_TARGET="${TARGET_USER}@${TARGET_HOST}"

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

if [ -n "$SSH_KEY" ]; then
  SSH_OPTS+=(-i "$SSH_KEY")
  SCP_OPTS+=(-i "$SSH_KEY")
fi

RSYNC_SSH="ssh -p $SSH_PORT -o ServerAliveInterval=30 -o ServerAliveCountMax=3"
if [ -n "$SSH_KEY" ]; then
  RSYNC_SSH="$RSYNC_SSH -i $SSH_KEY"
fi

RSYNC_OPTS=(
  -aH
  --numeric-ids
  --info=progress2
)

echo "[1/7] Checking target server"
ssh "${SSH_OPTS[@]}" "$REMOTE_TARGET" '
  set -eu
  command -v docker >/dev/null 2>&1 || { echo "ERROR: docker is not installed on target" >&2; exit 1; }
  docker compose version >/dev/null 2>&1 || { echo "ERROR: docker compose plugin is not available on target" >&2; exit 1; }
  command -v rsync >/dev/null 2>&1 || { echo "ERROR: rsync is not installed on target" >&2; exit 1; }
'

echo "[2/7] Saving images used by existing containers"
IMAGE_TAR="/tmp/gateway-docker-images-$(date +%Y%m%d%H%M%S).tar"
IMAGE_LIST="$(docker ps -a --format '{{.Image}}' | sed '/^$/d' | sort -u)"
VOLUME_LIST="$(collect_volume_names)"
if [ -n "$VOLUME_LIST" ]; then
  docker image inspect alpine:3.20 >/dev/null 2>&1 || docker pull alpine:3.20
  IMAGE_LIST="$(printf '%s\nalpine:3.20\n' "$IMAGE_LIST" | sed '/^$/d' | sort -u)"
fi
if [ -n "$IMAGE_LIST" ]; then
  # shellcheck disable=SC2086
  docker save -o "$IMAGE_TAR" $IMAGE_LIST
else
  echo "No containers found on source; skipping docker image export."
fi

echo "[3/7] Syncing gateway directory"
rsync_path "$GATEWAY_DIR/" "$TARGET_DIR/"

echo "[4/7] Syncing certificates"
if [ -d /etc/letsencrypt ]; then
  rsync_path "/etc/letsencrypt/" "/etc/letsencrypt/"
else
  echo "/etc/letsencrypt does not exist on source; skipping."
fi

echo "[5/7] Syncing docker bind mounts"
while IFS= read -r bind_source; do
  case "$bind_source" in
    "$GATEWAY_DIR"|"$GATEWAY_DIR"/*|/etc/letsencrypt|/etc/letsencrypt/*)
      continue
      ;;
  esac

  if [ -e "$bind_source" ]; then
    echo "Syncing bind mount: $bind_source"
    if [ -d "$bind_source" ]; then
      rsync_path "$bind_source/" "$bind_source/"
    else
      rsync_path "$bind_source" "$bind_source"
    fi
  fi
done <<EOF
$(collect_bind_sources)
EOF

echo "[6/7] Syncing docker named volumes"
while IFS= read -r volume_name; do
  if [ -z "$volume_name" ]; then
    continue
  fi

  volume_tar="/tmp/docker-volume-${volume_name}-$(date +%Y%m%d%H%M%S).tar.gz"
  remote_volume_tar="/tmp/$(basename "$volume_tar")"

  echo "Packing volume: $volume_name"
  docker run --rm -v "${volume_name}:/volume:ro" -v /tmp:/backup alpine:3.20 \
    tar -C /volume -czf "/backup/$(basename "$volume_tar")" .

  scp "${SCP_OPTS[@]}" "$volume_tar" "$REMOTE_TARGET:$remote_volume_tar"
  ssh "${SSH_OPTS[@]}" "$REMOTE_TARGET" "
    set -eu
    docker volume create $(remote_quote "$volume_name") >/dev/null
    docker run --rm -v $(remote_quote "$volume_name"):/volume -v /tmp:/backup alpine:3.20 sh -c 'cd /volume && tar -xzf /backup/$(basename "$remote_volume_tar")'
    rm -f $(remote_quote "$remote_volume_tar")
  "
  rm -f "$volume_tar"
done <<EOF
$VOLUME_LIST
EOF

echo "[7/7] Loading images and starting containers on target"
if [ -n "${IMAGE_LIST:-}" ]; then
  REMOTE_IMAGE_TAR="/tmp/$(basename "$IMAGE_TAR")"
  scp "${SCP_OPTS[@]}" "$IMAGE_TAR" "$REMOTE_TARGET:$REMOTE_IMAGE_TAR"
  ssh "${SSH_OPTS[@]}" "$REMOTE_TARGET" "docker load -i $(remote_quote "$REMOTE_IMAGE_TAR") && rm -f $(remote_quote "$REMOTE_IMAGE_TAR")"
  rm -f "$IMAGE_TAR"
fi

ssh "${SSH_OPTS[@]}" "$REMOTE_TARGET" "
  set -eu
  find $(remote_quote "$TARGET_DIR") -name docker-compose.yml -print | sort | while IFS= read -r compose_file; do
    docker compose -f \"\$compose_file\" down --remove-orphans >/dev/null 2>&1 || true
    docker compose -f \"\$compose_file\" up -d
  done
"

echo "Migration completed: $REMOTE_TARGET:$TARGET_DIR"
