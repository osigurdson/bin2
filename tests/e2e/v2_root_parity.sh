#!/usr/bin/env bash
set -euo pipefail

E2E_TEST_NAME=v2_root_parity
source "$(cd "$(dirname "$0")" && pwd)/lib.sh"

trap e2e_cleanup EXIT

e2e_require_core_env
e2e_require_cmd curl
e2e_require_cmd jq
e2e_require_cmd base64
e2e_setup_workdir

declare -A ROOT_STATUS
declare -A ROOT_API_VERSION
declare -A ROOT_CHALLENGE

run_root_request() {
  local key="$1"
  local base_url="$2"
  local method="$3"
  local auth_token="${4:-}"
  local headers_file="$E2E_WORKDIR/${key}-headers.txt"
  local body_file="$E2E_WORKDIR/${key}-body.txt"
  local status

  if [[ "$method" == "HEAD" ]]; then
    if [[ -n "$auth_token" ]]; then
      status="$(e2e_curl "$body_file" "$headers_file" --head --header "Authorization: Bearer ${auth_token}" "${base_url}/v2/")"
    else
      status="$(e2e_curl "$body_file" "$headers_file" --head "${base_url}/v2/")"
    fi
  else
    if [[ -n "$auth_token" ]]; then
      status="$(e2e_curl "$body_file" "$headers_file" --header "Authorization: Bearer ${auth_token}" "${base_url}/v2/")"
    else
      status="$(e2e_curl "$body_file" "$headers_file" "${base_url}/v2/")"
    fi
  fi

  ROOT_STATUS["$key"]="$status"
  ROOT_API_VERSION["$key"]="$(e2e_header_value "$headers_file" 'Docker-Distribution-API-Version')"
  ROOT_CHALLENGE["$key"]="$(e2e_header_value "$headers_file" 'WWW-Authenticate')"

  if [[ "$method" == "HEAD" ]] || [[ -n "$auth_token" ]]; then
    [[ ! -s "$body_file" ]] || e2e_fail "expected empty body for ${key}, got $(wc -c < "$body_file") bytes"
  fi
}

assert_unauth_root() {
  local key="$1"

  [[ "${ROOT_STATUS[$key]}" == "401" ]] || e2e_fail "expected 401 for ${key}, got ${ROOT_STATUS[$key]}"
  [[ "${ROOT_API_VERSION[$key]}" == "registry/2.0" ]] || e2e_fail "unexpected API version for ${key}: ${ROOT_API_VERSION[$key]}"
  [[ "${ROOT_CHALLENGE[$key]}" == Bearer* ]] || e2e_fail "expected bearer challenge for ${key}, got: ${ROOT_CHALLENGE[$key]}"
}

assert_auth_root() {
  local key="$1"

  [[ "${ROOT_STATUS[$key]}" == "200" ]] || e2e_fail "expected 200 for ${key}, got ${ROOT_STATUS[$key]}"
  [[ "${ROOT_API_VERSION[$key]}" == "registry/2.0" ]] || e2e_fail "unexpected API version for ${key}: ${ROOT_API_VERSION[$key]}"
  [[ -z "${ROOT_CHALLENGE[$key]}" ]] || e2e_fail "expected no auth challenge for ${key}, got: ${ROOT_CHALLENGE[$key]}"
}

fetch_root_token() {
  local challenge="$1"
  local realm
  local service

  realm="$(e2e_bearer_param "$challenge" realm)"
  service="$(e2e_bearer_param "$challenge" service)"
  [[ -n "$realm" ]] || e2e_fail 'missing bearer realm'
  [[ -n "$service" ]] || e2e_fail 'missing bearer service'

  e2e_fetch_token "$realm" "$service" "" "$E2E_PASSWORD"
}

push_base_url="$(e2e_push_base_url)"
pull_base_url="$(e2e_pull_base_url)"

run_root_request push-get-unauth "$push_base_url" GET
run_root_request pull-get-unauth "$pull_base_url" GET
assert_unauth_root push-get-unauth
assert_unauth_root pull-get-unauth
[[ "${ROOT_STATUS[push-get-unauth]}" == "${ROOT_STATUS[pull-get-unauth]}" ]] || e2e_fail "push/pull GET /v2/ unauth status mismatch"
[[ "${ROOT_API_VERSION[push-get-unauth]}" == "${ROOT_API_VERSION[pull-get-unauth]}" ]] || e2e_fail "push/pull GET /v2/ unauth API version mismatch"

run_root_request push-head-unauth "$push_base_url" HEAD
run_root_request pull-head-unauth "$pull_base_url" HEAD
assert_unauth_root push-head-unauth
assert_unauth_root pull-head-unauth
[[ "${ROOT_STATUS[push-head-unauth]}" == "${ROOT_STATUS[pull-head-unauth]}" ]] || e2e_fail "push/pull HEAD /v2/ unauth status mismatch"
[[ "${ROOT_API_VERSION[push-head-unauth]}" == "${ROOT_API_VERSION[pull-head-unauth]}" ]] || e2e_fail "push/pull HEAD /v2/ unauth API version mismatch"

push_token="$(fetch_root_token "${ROOT_CHALLENGE[push-get-unauth]}")"
pull_token="$(fetch_root_token "${ROOT_CHALLENGE[pull-get-unauth]}")"

run_root_request push-get-auth "$push_base_url" GET "$push_token"
run_root_request pull-get-auth "$pull_base_url" GET "$pull_token"
assert_auth_root push-get-auth
assert_auth_root pull-get-auth
[[ "${ROOT_STATUS[push-get-auth]}" == "${ROOT_STATUS[pull-get-auth]}" ]] || e2e_fail "push/pull GET /v2/ auth status mismatch"
[[ "${ROOT_API_VERSION[push-get-auth]}" == "${ROOT_API_VERSION[pull-get-auth]}" ]] || e2e_fail "push/pull GET /v2/ auth API version mismatch"

run_root_request push-head-auth "$push_base_url" HEAD "$push_token"
run_root_request pull-head-auth "$pull_base_url" HEAD "$pull_token"
assert_auth_root push-head-auth
assert_auth_root pull-head-auth
[[ "${ROOT_STATUS[push-head-auth]}" == "${ROOT_STATUS[pull-head-auth]}" ]] || e2e_fail "push/pull HEAD /v2/ auth status mismatch"
[[ "${ROOT_API_VERSION[push-head-auth]}" == "${ROOT_API_VERSION[pull-head-auth]}" ]] || e2e_fail "push/pull HEAD /v2/ auth API version mismatch"

e2e_log "validated GET/HEAD /v2/ parity across ${push_base_url} and ${pull_base_url}"
