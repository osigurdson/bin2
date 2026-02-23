#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 1 ]]; then
	echo "usage: $0 <tag>"
	exit 1
fi

: "${VULTR_CR_USERNAME:?VULTR_CR_USERNAME is required}"
: "${VULTR_CR_PASSWORD:?VULTR_CR_PASSWORD is required}"

podman login https://lax.vultrcr.com/bin2 -u "$VULTR_CR_USERNAME" -p "$VULTR_CR_PASSWORD"

go build .
podman build -t init:"$1" .
podman tag init:"$1" lax.vultrcr.com/bin2/init:"$1"
podman push lax.vultrcr.com/bin2/init:"$1"
