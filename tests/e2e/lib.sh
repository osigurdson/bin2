#!/usr/bin/env bash
set -euo pipefail

readonly E2E_LIB_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly E2E_REPO_ROOT="$(cd "${E2E_LIB_DIR}/../.." && pwd)"
readonly E2E_REGISTRY_LOGIN_USER="bin2"

: "${E2E_PUSH_REGISTRY:=localhost:5000}"
: "${E2E_PULL_REGISTRY:=${E2E_PUSH_REGISTRY}}"
: "${E2E_NAMESPACE:=${REGISTRY_NS:-}}"
: "${E2E_PASSWORD:=${REGISTRY_PW:-}}"
: "${E2E_PUSH_SCHEME:=${E2E_SCHEME:-http}}"
: "${E2E_PULL_SCHEME:=${E2E_PUSH_SCHEME}}"
: "${E2E_SOURCE_IMAGE:=docker.io/library/hello-world:latest}"
: "${E2E_HTTP_TIMEOUT:=30}"
: "${E2E_RUN_ID:=$(date +%Y%m%d%H%M%S)-$$}"

export E2E_PUSH_REGISTRY E2E_PULL_REGISTRY E2E_NAMESPACE E2E_PASSWORD
export E2E_PUSH_SCHEME E2E_PULL_SCHEME
export E2E_SOURCE_IMAGE E2E_HTTP_TIMEOUT E2E_RUN_ID


e2e_log() {
  printf '[%s] %s\n' "${E2E_TEST_NAME:-e2e}" "$*"
}


e2e_fail() {
  printf '[%s] ERROR: %s\n' "${E2E_TEST_NAME:-e2e}" "$*" >&2
  exit 1
}


e2e_require_cmd() {
  local cmd="$1"
  command -v "$cmd" >/dev/null 2>&1 || e2e_fail "missing command: $cmd"
}


e2e_require_env() {
  local name="$1"
  [[ -n "${!name:-}" ]] || e2e_fail "$name is required"
}


e2e_require_core_env() {
  e2e_require_env E2E_NAMESPACE
  e2e_require_env E2E_PASSWORD
}


e2e_setup_workdir() {
  E2E_WORKDIR="$(mktemp -d "${TMPDIR:-/tmp}/${E2E_TEST_NAME:-bin2-e2e}.${E2E_RUN_ID}.XXXXXX")"
  export E2E_WORKDIR
}


e2e_cleanup() {
  if [[ -n "${E2E_WORKDIR:-}" ]] && [[ -d "${E2E_WORKDIR}" ]]; then
    rm -rf "${E2E_WORKDIR}"
  fi
}


e2e_push_base_url() {
  printf '%s://%s' "$E2E_PUSH_SCHEME" "$E2E_PUSH_REGISTRY"
}


e2e_pull_base_url() {
  printf '%s://%s' "$E2E_PULL_SCHEME" "$E2E_PULL_REGISTRY"
}


e2e_repo_path() {
  printf '%s/%s' "$E2E_NAMESPACE" "$1"
}


e2e_push_ref() {
  local repo_leaf="$1"
  local ref="$2"
  printf '%s/%s:%s' "$E2E_PUSH_REGISTRY" "$(e2e_repo_path "$repo_leaf")" "$ref"
}


e2e_pull_ref() {
  local repo_leaf="$1"
  local ref="$2"
  printf '%s/%s:%s' "$E2E_PULL_REGISTRY" "$(e2e_repo_path "$repo_leaf")" "$ref"
}


e2e_pull_digest_ref() {
  local repo_leaf="$1"
  local digest="$2"
  printf '%s/%s@%s' "$E2E_PULL_REGISTRY" "$(e2e_repo_path "$repo_leaf")" "$digest"
}


e2e_unique_repo() {
  local prefix="$1"
  prefix="${prefix//[^A-Za-z0-9._-]/-}"
  printf '%s-%s' "$prefix" "$E2E_RUN_ID"
}


e2e_header_value() {
  local header_file="$1"
  local name="$2"
  awk -v wanted="$(printf '%s' "$name" | tr '[:upper:]' '[:lower:]')" '
    BEGIN { FS = ":" }
    {
      key = tolower($1)
      if (key == wanted) {
        sub(/^:[[:space:]]*/, "", $0)
        sub(/^[^:]*:[[:space:]]*/, "", $0)
        sub(/\r$/, "", $0)
        print $0
        exit
      }
    }
  ' "$header_file"
}


e2e_bearer_param() {
  local challenge="$1"
  local key="$2"
  printf '%s' "$challenge" | sed -nE "s/.*${key}=\"([^\"]*)\".*/\\1/p"
}


e2e_curl() {
  local body_file="$1"
  local header_file="$2"
  shift 2
  curl --silent --show-error \
    --max-time "$E2E_HTTP_TIMEOUT" \
    --dump-header "$header_file" \
    --output "$body_file" \
    --write-out '%{http_code}' \
    "$@"
}


e2e_base64url_decode() {
  local data="$1"
  data="${data//-/+}"
  data="${data//_/\/}"
  case "$(( ${#data} % 4 ))" in
    0) ;;
    2) data+="==" ;;
    3) data+="=" ;;
    *) e2e_fail "invalid base64url payload" ;;
  esac
  printf '%s' "$data" | base64 --decode
}


e2e_jwt_payload_json() {
  local token="$1"
  local payload="${token#*.}"
  payload="${payload%%.*}"
  e2e_base64url_decode "$payload"
}


e2e_token_actions() {
  local payload_json="$1"
  local repo="$2"
  printf '%s' "$payload_json" |
    jq -r --arg repo "$repo" '.access[]? | select(.type == "repository" and .name == $repo) | .actions[]' |
    sort |
    paste -sd ',' -
}


e2e_fetch_token() {
  local realm="$1"
  local service="$2"
  local scope="$3"
  local password="$4"
  local body_file="$E2E_WORKDIR/token-body.json"
  local header_file="$E2E_WORKDIR/token-headers.txt"
  local status

  status="$(e2e_curl "$body_file" "$header_file" \
    --user "${E2E_REGISTRY_LOGIN_USER}:${password}" \
    --get \
    --data-urlencode "service=${service}" \
    --data-urlencode "scope=${scope}" \
    "$realm")"
  [[ "$status" == "200" ]] || e2e_fail "token request failed with status ${status}: $(cat "$body_file")"

  jq -r '.token' "$body_file"
}


e2e_manifest_head() {
  local base_url="$1"
  local repo="$2"
  local reference="$3"
  local token="$4"
  local header_file="$5"
  local body_file="$6"

  e2e_curl "$body_file" "$header_file" \
    --head \
    --header "Authorization: Bearer ${token}" \
    "${base_url}/v2/${repo}/manifests/${reference}"
}
