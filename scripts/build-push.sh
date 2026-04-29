#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

REGISTRY="192.168.1.101:5000"
IMAGE_NAME="gateway/new-api"
TAG="latest"

# 默认架构 amd64，可通过参数传入：./build-push.sh arm64
TARGETARCH="${1:-amd64}"

FULL_IMAGE="${REGISTRY}/${IMAGE_NAME}:${TAG}"

cd "$PROJECT_DIR"

echo "Building Docker image for ${TARGETARCH}..."
docker build \
  --platform linux/${TARGETARCH} \
  --build-arg TARGETARCH=${TARGETARCH} \
  -t ${IMAGE_NAME}:${TAG} .

echo "Tagging image as ${FULL_IMAGE}..."
docker tag ${IMAGE_NAME}:${TAG} ${FULL_IMAGE}

echo "Pushing to registry..."
docker push ${FULL_IMAGE}

echo "Removing local images..."
docker rmi ${FULL_IMAGE} || true
docker rmi ${IMAGE_NAME}:${TAG} || true

echo "Calling remote update script..."
ssh root@192.168.1.100 "new-api-update.sh"

echo "Done! Image: ${FULL_IMAGE}"
