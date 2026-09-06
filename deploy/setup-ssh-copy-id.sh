#!/usr/bin/env bash
set -eu

usage() {
  cat >&2 <<'USAGE'
Usage:
  ./setup-ssh-copy-id.sh [target-user] [target-host...]

Environment:
  SSH_PORT=22
  SSH_KEY=/root/.ssh/id_ed25519_gateway_migrate

Example:
  ./setup-ssh-copy-id.sh
  ./setup-ssh-copy-id.sh root 203.0.113.10 203.0.113.11
USAGE
}

require_command() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "ERROR: required command not found: $1" >&2
    exit 1
  fi
}

write_ssh_config_entry() {
  target_host="$1"
  config_file="$HOME/.ssh/config"
  tmp_file="${config_file}.tmp.$$"
  begin_marker="# gateway-migrate ${target_host} begin"
  end_marker="# gateway-migrate ${target_host} end"

  touch "$config_file"
  chmod 600 "$config_file"

  awk -v begin="$begin_marker" -v end="$end_marker" '
    $0 == begin { skip = 1; next }
    $0 == end { skip = 0; next }
    skip != 1 { print }
  ' "$config_file" > "$tmp_file"

  {
    cat "$tmp_file"
    printf '%s\n' "$begin_marker"
    printf 'Host %s\n' "$target_host"
    printf '  HostName %s\n' "$target_host"
    printf '  User %s\n' "$TARGET_USER"
    printf '  Port %s\n' "$SSH_PORT"
    printf '  IdentityFile %s\n' "$SSH_KEY"
    printf '  IdentitiesOnly yes\n'
    printf '%s\n' "$end_marker"
  } > "$config_file"

  rm -f "$tmp_file"
  chmod 600 "$config_file"
}

TARGET_USER="${1:-}"
SSH_PORT="${SSH_PORT:-22}"
SSH_KEY="${SSH_KEY:-$HOME/.ssh/id_ed25519_gateway_migrate}"

if [ "${1:-}" = "-h" ] || [ "${1:-}" = "--help" ]; then
  usage
  exit 0
fi

if [ -z "$TARGET_USER" ]; then
  printf "Target SSH user [root]: "
  read -r TARGET_USER
  TARGET_USER="${TARGET_USER:-root}"
fi

if [ "$#" -gt 0 ]; then
  shift
fi
TARGET_HOSTS="$*"

if [ -z "$TARGET_HOSTS" ]; then
  printf "Target server IPs, separated by spaces or commas: "
  read -r TARGET_HOSTS
fi

printf "Target SSH port [%s]: " "$SSH_PORT"
read -r INPUT_SSH_PORT
SSH_PORT="${INPUT_SSH_PORT:-$SSH_PORT}"

TARGET_HOSTS="$(printf '%s\n' "$TARGET_HOSTS" | tr ',' ' ')"

if [ -z "$TARGET_HOSTS" ]; then
  echo "ERROR: at least one target server IP is required" >&2
  exit 1
fi

require_command ssh
require_command ssh-keygen
require_command ssh-copy-id

mkdir -p "$(dirname "$SSH_KEY")"
chmod 700 "$(dirname "$SSH_KEY")"

if [ -f "$SSH_KEY" ]; then
  echo "SSH key already exists: $SSH_KEY"
else
  echo "Generating SSH key: $SSH_KEY"
  ssh-keygen -t ed25519 -f "$SSH_KEY" -N "" -C "gateway-migrate@$(hostname)"
fi

for TARGET_HOST in $TARGET_HOSTS; do
  write_ssh_config_entry "$TARGET_HOST"

  echo "Copying public key to ${TARGET_USER}@${TARGET_HOST}:${SSH_PORT}"
  ssh-copy-id -i "${SSH_KEY}.pub" -p "$SSH_PORT" "${TARGET_USER}@${TARGET_HOST}"

  echo "Testing passwordless SSH login: ${TARGET_USER}@${TARGET_HOST}"
  ssh \
    -o BatchMode=yes \
    -o ConnectTimeout=8 \
    "$TARGET_HOST" \
    "echo SSH login OK: \$(hostname)"
done

echo "Done."
echo "You can run migration with:"
for TARGET_HOST in $TARGET_HOSTS; do
  echo "  SSH_KEY=$SSH_KEY SSH_PORT=$SSH_PORT ./migrate-to-server.sh $TARGET_HOST $TARGET_USER"
done
echo "You can also SSH directly with:"
for TARGET_HOST in $TARGET_HOSTS; do
  echo "  ssh $TARGET_HOST"
done
