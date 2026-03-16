#!/usr/bin/env bash
set -euo pipefail

readonly SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
readonly REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
readonly DOTENV_FILE="${REPO_ROOT}/.env"

usage() {
	cat <<'EOF'
Usage: ./tests/run-registry-matrix.sh [PUSH_ROOT_URL] [PULL_ROOT_URL]

Runs two verification passes:
  1. Full OCI distribution conformance against the push-capable Go API.
  2. Split-host e2e coverage that pushes via the Go API and pulls via the
     read-only pull endpoint.

Defaults:
  PUSH_ROOT_URL = https://bin2.io
  PULL_ROOT_URL = https://pull.bin2.io

Environment:
  E2E_NAMESPACE or BIN2_SEED_E2E_REGISTRY must be set.
  E2E_PASSWORD or BIN2_SEED_E2E_API_KEY must be set.
EOF
}

if [[ "${1:-}" == "--help" ]] || [[ "${1:-}" == "-h" ]]; then
	usage
	exit 0
fi

if [[ -f "${DOTENV_FILE}" ]]; then
	set -a
	# shellcheck disable=SC1090
	source "${DOTENV_FILE}"
	set +a
fi

push_root_url="${1:-https://bin2.io}"
pull_root_url="${2:-https://pull.bin2.io}"
namespace="${BIN2_SEED_E2E_REGISTRY:-${E2E_NAMESPACE:-}}"
password="${BIN2_SEED_E2E_API_KEY:-${E2E_PASSWORD:-}}"

if [[ -z "${namespace}" ]]; then
	printf 'set E2E_NAMESPACE or BIN2_SEED_E2E_REGISTRY\n' >&2
	exit 1
fi
if [[ -z "${password}" ]]; then
	printf 'set E2E_PASSWORD or BIN2_SEED_E2E_API_KEY\n' >&2
	exit 1
fi

parse_root_url() {
	local url="$1"
	local host
	local scheme

	case "$url" in
	http://*)
		scheme="http"
		host="${url#http://}"
		;;
	https://*)
		scheme="https"
		host="${url#https://}"
		;;
	*)
		printf 'root URL must include scheme: %s\n' "$url" >&2
		exit 1
		;;
	esac

	host="${host%%/*}"
	if [[ -z "${host}" ]]; then
		printf 'root URL host is empty: %s\n' "$url" >&2
		exit 1
	fi

	printf '%s %s\n' "$scheme" "$host"
}

read -r push_scheme push_host < <(parse_root_url "${push_root_url}")
read -r pull_scheme pull_host < <(parse_root_url "${pull_root_url}")

: "${E2E_RUN_ID:=$(date +%Y%m%d%H%M%S)-$$}"
export E2E_RUN_ID

conformance_namespace="${namespace}/conformance-${E2E_RUN_ID}"

split_tests=(
	auth_handshake
	oras_artifact_roundtrip
	image_interop_roundtrip
	pull_negative_paths
)

printf '==> Pass 1: OCI conformance against %s\n' "${push_root_url}"
(
	unset \
		OCI_ROOT_URL \
		OCI_NAMESPACE \
		OCI_CROSSMOUNT_NAMESPACE \
		OCI_USERNAME \
		OCI_AUTOMATIC_CROSSMOUNT \
		OCI_TAG_NAME \
		OCI_MANIFEST_DIGEST \
		OCI_BLOB_DIGEST \
		OCI_TAG_LIST \
		OCI_TEST_PULL \
		OCI_TEST_PUSH \
		OCI_TEST_CONTENT_DISCOVERY \
		OCI_TEST_CONTENT_MANAGEMENT \
		OCI_HIDE_SKIPPED_WORKFLOWS \
		OCI_DEBUG \
		OCI_REPORT_DIR

	export OCI_USERNAME="bin2"
	export OCI_PASSWORD="${password}"
	export OCI_TEST_PULL="1"
	export OCI_TEST_PUSH="1"
	export OCI_TEST_CONTENT_DISCOVERY="1"
	export OCI_TEST_CONTENT_MANAGEMENT="1"
	export OCI_HIDE_SKIPPED_WORKFLOWS="1"
	export OCI_DEBUG="0"

	"${SCRIPT_DIR}/oci-conformance/run-oci-conformance.sh" \
		--root-url "${push_root_url}" \
		--namespace "${conformance_namespace}"
)

printf '\n==> Pass 2: split-host e2e (push=%s pull=%s)\n' "${push_root_url}" "${pull_root_url}"
(
	export E2E_NAMESPACE="${namespace}"
	export E2E_PASSWORD="${password}"
	export E2E_PUSH_REGISTRY="${push_host}"
	export E2E_PULL_REGISTRY="${pull_host}"
	export E2E_PUSH_SCHEME="${push_scheme}"
	export E2E_PULL_SCHEME="${pull_scheme}"
	"${SCRIPT_DIR}/run-e2e.sh" "${split_tests[@]}"
)
