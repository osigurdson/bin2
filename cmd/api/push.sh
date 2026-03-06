#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 0 ]]; then
	echo "usage: $0"
	exit 1
fi

: "${VULTR_CR_USERNAME:?VULTR_CR_USERNAME is required}"
: "${VULTR_CR_PASSWORD:?VULTR_CR_PASSWORD is required}"

count_file=".pushcount"

if [[ ! -f "$count_file" ]]; then
	echo "missing $count_file"
	exit 1
fi

tag="$(tr -d '[:space:]' < "$count_file")"
if [[ ! "$tag" =~ ^[0-9]+$ ]]; then
	echo "invalid $count_file value: $tag"
	exit 1
fi

podman login https://lax.vultrcr.com/bin2 -u "$VULTR_CR_USERNAME" -p "$VULTR_CR_PASSWORD"

go build .
echo "building localhost/api:$tag"
podman build --quiet -t api:"$tag" .
podman tag api:"$tag" lax.vultrcr.com/bin2/api:"$tag"
echo "pushing lax.vultrcr.com/bin2/api:$tag"
podman push lax.vultrcr.com/bin2/api:"$tag"

echo $((tag + 1)) > "$count_file"
