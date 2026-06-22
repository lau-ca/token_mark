#!/usr/bin/env bash
set -eu

ROLE="${1:-all}"
GATEWAY_DIR="${GATEWAY_DIR:-/root/gateway}"
IMAGE_TAR="${IMAGE_TAR:-${GATEWAY_DIR}/new-api-latest.tar}"
IMAGE_TAG="${IMAGE_TAG:-calciumion/new-api:latest}"

load_image() {
  if [ -f "$IMAGE_TAR" ]; then
    docker rmi "$IMAGE_TAG" >/dev/null 2>&1 || true
    docker load -i "$IMAGE_TAR"
  fi
}

up_master() {
  docker compose -f "${GATEWAY_DIR}/master/docker-compose.yml" up -d
  docker compose -f "${GATEWAY_DIR}/nginx/docker-compose.yml" up -d
}

up_worker() {
  docker compose -f "${GATEWAY_DIR}/work/docker-compose.yml" up -d
}

case "$ROLE" in
  all|both)
    load_image
    up_master
    up_worker
    ;;
  master)
    load_image
    up_master
    ;;
  worker|work|slave)
    load_image
    up_worker
    ;;
  *)
    echo "Usage: $0 [all|master|worker]" >&2
    exit 1
    ;;
esac
