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

env_local_tmp="__env_local"

if [[ -e "$env_local_tmp" ]]; then
	echo "temporary env file already exists: $env_local_tmp"
	exit 1
fi

restore_env_local() {
	if [[ -f "$env_local_tmp" ]]; then
		mv "$env_local_tmp" .env.local
	fi
}

trap restore_env_local EXIT

if [[ -f .env.local ]]; then
	mv .env.local "$env_local_tmp"
fi

podman login https://lax.vultrcr.com/bin2 -u "$VULTR_CR_USERNAME" -p "$VULTR_CR_PASSWORD"

NODE_ENV=production npm run build
podman build -t ui:"$tag" .
podman tag ui:"$tag" lax.vultrcr.com/bin2/ui:"$tag"
podman push lax.vultrcr.com/bin2/ui:"$tag"

echo $((tag + 1)) > "$count_file"
