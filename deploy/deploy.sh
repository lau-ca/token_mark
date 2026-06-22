#!/usr/bin/env bash
set -euo pipefail

# Build, tag, and save docker image for new-api
# Usage:
#   ./deploy.sh [output-tar]
# Example:
#   ./deploy.sh artifacts/new-api-latest.tar

IMAGE_TAG="calciumion/new-api:latest"
OUTPUT_TAR="${1:-artifacts/new-api-latest.tar}"
TARGET_PLATFORM="${TARGET_PLATFORM:-linux/amd64}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

if [[ -n "${VITE_REACT_APP_SERVER_URL:-}" ]]; then
  cat >&2 <<'EOF'
ERROR: VITE_REACT_APP_SERVER_URL must not be set when building a production image.
It is embedded into frontend assets and can cause browser-side CORS issues.
For local classic UI development against production, use:
  cd web/classic
  DEV_PROXY_SERVER_URL=https://token.xmodel.chat bun run dev
EOF
  exit 1
fi

if [[ -n "${DEV_PROXY_SERVER_URL:-}" ]]; then
  echo "Note: DEV_PROXY_SERVER_URL is only used by the local Vite dev server and is ignored by production builds."
fi

# Save tar in deploy/ by default when relative path is provided
if [[ "${OUTPUT_TAR}" != /* ]]; then
  OUTPUT_TAR="${SCRIPT_DIR}/${OUTPUT_TAR}"
fi
mkdir -p "$(dirname "${OUTPUT_TAR}")"

echo "[1/3] Building image: ${IMAGE_TAG} (${TARGET_PLATFORM})"
docker build --platform "${TARGET_PLATFORM}" -t "${IMAGE_TAG}" "${PROJECT_ROOT}"

echo "[2/3] Tagging image: ${IMAGE_TAG}"
# Keep explicit tag step as required workflow.
docker tag "${IMAGE_TAG}" "${IMAGE_TAG}"

echo "[3/3] Saving image to: ${OUTPUT_TAR}"
docker save -o "${OUTPUT_TAR}" "${IMAGE_TAG}"

echo "Done: ${IMAGE_TAG} (${TARGET_PLATFORM}) -> ${OUTPUT_TAR}"
