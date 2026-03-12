#!/usr/bin/env bash
set -euo pipefail

readonly SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
readonly REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

usage() {
  cat <<'EOF'
Usage: ./tests/oci-conformance/run-oci-conformance.sh [ROOT_URL] [NAMESPACE] [options]

Options:
  --namespace NAME  OCI namespace to test. Example: myregistry/conformance-main.
  --root-url URL    Registry root URL. Defaults to OCI_ROOT_URL or http://localhost:5000.
  --ref REF         opencontainers/distribution-spec git ref. Default: v1.1.1.
  --help            Show this help text.

Defaults:
  - Runs the full OCI distribution suite.
  - Uses OCI_USERNAME=bin2 unless overridden.
  - Prompts for OCI_PASSWORD if not exported.
  - Derives OCI_CROSSMOUNT_NAMESPACE automatically if not set.
EOF
}

namespace="${OCI_NAMESPACE:-}"
root_url="${OCI_ROOT_URL:-http://localhost:5000}"
suite_ref="${OCI_CONFORMANCE_REF:-v1.1.1}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --namespace)
      namespace="${2:?missing value for --namespace}"
      shift 2
      ;;
    --root-url)
      root_url="${2:?missing value for --root-url}"
      shift 2
      ;;
    --ref)
      suite_ref="${2:?missing value for --ref}"
      shift 2
      ;;
    --help|-h)
      usage
      exit 0
      ;;
    --*)
      printf 'unknown argument: %s\n' "$1" >&2
      usage >&2
      exit 1
      ;;
    *)
      if [[ "${root_url}" == "${OCI_ROOT_URL:-http://localhost:5000}" ]]; then
        root_url="$1"
      elif [[ -z "${namespace}" ]]; then
        namespace="$1"
      else
        printf 'unexpected argument: %s\n' "$1" >&2
        usage >&2
        exit 1
      fi
      shift
      ;;
  esac
done

export OCI_ROOT_URL="${root_url}"

: "${OCI_ROOT_URL:?set OCI_ROOT_URL}"
if [[ -n "${namespace}" ]]; then
  export OCI_NAMESPACE="${namespace}"
fi
: "${OCI_NAMESPACE:?set OCI_NAMESPACE or pass NAMESPACE}"
: "${OCI_USERNAME:=bin2}"
if [[ -z "${OCI_CROSSMOUNT_NAMESPACE:-}" ]]; then
  if [[ "${OCI_NAMESPACE}" != */* ]]; then
    OCI_CROSSMOUNT_NAMESPACE="${OCI_NAMESPACE}-crossmount"
  else
    namespace_prefix="${OCI_NAMESPACE%/*}"
    namespace_leaf="${OCI_NAMESPACE##*/}"
    OCI_CROSSMOUNT_NAMESPACE="${namespace_prefix}/${namespace_leaf}-crossmount"
  fi
  export OCI_CROSSMOUNT_NAMESPACE
fi
if [[ -z "${OCI_PASSWORD:-}" ]]; then
  if [[ -t 0 ]]; then
    read -r -s -p "OCI_PASSWORD (API key): " OCI_PASSWORD
    printf '\n'
    export OCI_PASSWORD
  else
    printf 'set OCI_PASSWORD or run interactively to paste an API key\n' >&2
    exit 1
  fi
fi

: "${OCI_TEST_PULL:=1}"
: "${OCI_TEST_PUSH:=1}"
: "${OCI_TEST_CONTENT_DISCOVERY:=1}"
: "${OCI_TEST_CONTENT_MANAGEMENT:=1}"
: "${OCI_HIDE_SKIPPED_WORKFLOWS:=1}"
: "${OCI_DEBUG:=0}"
: "${OCI_REPORT_DIR:=test-results/oci-conformance}"
: "${OCI_AUTOMATIC_CROSSMOUNT:=true}"

export OCI_ROOT_URL
export OCI_NAMESPACE
export OCI_CROSSMOUNT_NAMESPACE
export OCI_USERNAME
export OCI_PASSWORD
export OCI_AUTOMATIC_CROSSMOUNT
export OCI_TEST_PULL
export OCI_TEST_PUSH
export OCI_TEST_CONTENT_DISCOVERY
export OCI_TEST_CONTENT_MANAGEMENT
export OCI_HIDE_SKIPPED_WORKFLOWS
export OCI_DEBUG
export OCI_REPORT_DIR

cache_root="${OCI_CONFORMANCE_CACHE_DIR:-${REPO_ROOT}/tests/.cache}"
suite_dir="${cache_root}/distribution-spec"
binary_path="${cache_root}/conformance-${suite_ref}.test"

mkdir -p "${cache_root}"
if [[ ! -d "${suite_dir}/.git" ]]; then
  git clone https://github.com/opencontainers/distribution-spec.git "${suite_dir}" >/dev/null
fi

git -C "${suite_dir}" fetch --tags origin >/dev/null
git -C "${suite_dir}" checkout --detach "${suite_ref}" >/dev/null

(
  cd "${suite_dir}/conformance"
  go test -c -o "${binary_path}"
)

if [[ "${OCI_REPORT_DIR}" != "none" ]]; then
  mkdir -p "${REPO_ROOT}/${OCI_REPORT_DIR}"
fi

printf 'Running OCI conformance from %s against %s\n' "${suite_ref}" "${OCI_ROOT_URL}"
printf 'Namespace: %s\n' "${OCI_NAMESPACE}"
printf 'Crossmount Namespace: %s\n' "${OCI_CROSSMOUNT_NAMESPACE}"
printf 'Workflows: pull, push, content-discovery, content-management\n'
if [[ "${OCI_REPORT_DIR}" != "none" ]]; then
  printf 'Reports: %s\n' "${OCI_REPORT_DIR}"
fi

cd "${REPO_ROOT}"
"${binary_path}"
