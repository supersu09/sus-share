#!/bin/sh
set -eu

IMAGE_NAME="${SUS_SHARE_IMAGE_NAME:-sus-share}"
IMAGE_TAG="${SUS_SHARE_IMAGE_TAG:-0.1.1}"
TARGET_CONTEXT="${SUS_SHARE_DOCKER_CONTEXT:-winserver}"
PLATFORM="${SUS_SHARE_PLATFORM:-linux/amd64}"
GOPROXY="${SUS_SHARE_GOPROXY:-https://goproxy.cn,direct}"

BUILD_FLAGS=
if [ "${SUS_SHARE_NO_CACHE:-0}" = "1" ]; then
  BUILD_FLAGS="--no-cache"
fi

docker --context "$TARGET_CONTEXT" build $BUILD_FLAGS \
  --platform "$PLATFORM" \
  --build-arg "GOPROXY=$GOPROXY" \
  -t "$IMAGE_NAME:$IMAGE_TAG" \
  -t "$IMAGE_NAME:latest" \
  .

IMAGE_ID="$(docker --context "$TARGET_CONTEXT" image inspect \
  "$IMAGE_NAME:$IMAGE_TAG" --format '{{.Id}}')"
IMAGE_ARCH="$(docker --context "$TARGET_CONTEXT" image inspect \
  "$IMAGE_NAME:$IMAGE_TAG" --format '{{.Architecture}}')"

printf 'Built %s:%s\nImage: %s\nArchitecture: %s\nDocker context: %s\n' \
  "$IMAGE_NAME" "$IMAGE_TAG" "$IMAGE_ID" "$IMAGE_ARCH" "$TARGET_CONTEXT"
