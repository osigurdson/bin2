#!/usr/bin/env bash
set -euo pipefail

REGISTRY_ADDR="${REGISTRY_ADDR:-localhost:5000}"
REGISTRY_NS="${REGISTRY_NS:-nthesis}"
REGISTRY_USER="${REGISTRY_USER:-bin2}"
TLS_VERIFY="${TLS_VERIFY:---tls-verify=false}"
TLS_ARGS=()
if [[ -n "$TLS_VERIFY" ]]; then
  TLS_ARGS+=("$TLS_VERIFY")
fi

IMAGE_INPUT="${1:-}"
if [[ -z "$IMAGE_INPUT" ]]; then
  read -rp "Docker Hub image (e.g. node:latest or pgvector/pgvector:pg18): " IMAGE_INPUT
fi

IMAGE_INPUT="${IMAGE_INPUT#docker.io/}"
if [[ -z "$IMAGE_INPUT" ]]; then
  echo "No image supplied."
  exit 1
fi

if [[ "$IMAGE_INPUT" != *@* && "$IMAGE_INPUT" != *:* ]]; then
  IMAGE_INPUT="${IMAGE_INPUT}:latest"
fi

if [[ "$IMAGE_INPUT" == */* ]]; then
  SOURCE_IMAGE="docker.io/${IMAGE_INPUT}"
  TARGET_IMAGE="${IMAGE_INPUT}"
else
  SOURCE_IMAGE="docker.io/library/${IMAGE_INPUT}"
  TARGET_IMAGE="${IMAGE_INPUT}"
fi

if [[ -z "${REGISTRY_PW:-}" ]]; then
  read -rsp "Registry password for ${REGISTRY_USER}: " REGISTRY_PW
  echo
fi

echo "Pulling ${SOURCE_IMAGE}"
podman pull "${SOURCE_IMAGE}"

echo "Logging in to ${REGISTRY_ADDR} as ${REGISTRY_USER}"
podman login "${TLS_ARGS[@]}" "${REGISTRY_ADDR}" -u "${REGISTRY_USER}" -p "${REGISTRY_PW}"

DEST_IMAGE="${REGISTRY_ADDR}/${REGISTRY_NS}/${TARGET_IMAGE}"
echo "Tagging ${SOURCE_IMAGE} -> ${DEST_IMAGE}"
podman tag "${SOURCE_IMAGE}" "${DEST_IMAGE}"

echo "Pushing ${DEST_IMAGE}"
podman push "${TLS_ARGS[@]}" "${DEST_IMAGE}"
