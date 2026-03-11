#!/usr/bin/env bash
set -euo pipefail

readonly SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
readonly COUNT_FILE="${SCRIPT_DIR}/.pushcount"
readonly IMAGE_REPO="lax.vultrcr.com/bin2/api"

if [[ $# -ne 0 ]]; then
	echo "usage: $0"
	exit 1
fi

: "${VULTR_CR_USERNAME:?VULTR_CR_USERNAME is required}"
: "${VULTR_CR_PASSWORD:?VULTR_CR_PASSWORD is required}"

if [[ ! -f "$COUNT_FILE" ]]; then
	echo "missing $COUNT_FILE"
	exit 1
fi

tag="$(tr -d '[:space:]' < "$COUNT_FILE")"
if [[ ! "$tag" =~ ^[0-9]+$ ]]; then
	echo "invalid $COUNT_FILE value: $tag"
	exit 1
fi

build_dir="$(mktemp -d)"
trap 'rm -rf "$build_dir"' EXIT

podman login https://lax.vultrcr.com/bin2 -u "$VULTR_CR_USERNAME" -p "$VULTR_CR_PASSWORD"

(
	cd "$REPO_ROOT"
	go build -o "${build_dir}/api" ./cmd/api
)
cp "${SCRIPT_DIR}/Dockerfile" "${build_dir}/Dockerfile"

echo "building ${IMAGE_REPO}:$tag"
podman build --quiet -t api:"$tag" "$build_dir"
podman tag api:"$tag" "${IMAGE_REPO}:$tag"
echo "pushing ${IMAGE_REPO}:$tag"
podman push "${IMAGE_REPO}:$tag"

echo $((tag + 1)) > "$COUNT_FILE"
