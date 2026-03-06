#!/usr/bin/env bash
set -euo pipefail

E2E_TEST_NAME=auth_handshake
source "$(cd "$(dirname "$0")" && pwd)/lib.sh"

trap e2e_cleanup EXIT

e2e_require_core_env
e2e_require_cmd curl
e2e_require_cmd jq
e2e_require_cmd base64
e2e_setup_workdir

base_url="$(e2e_pull_base_url)"
repo="$(e2e_repo_path "$(e2e_unique_repo auth)")"
scope="repository:${repo}:pull,push"
headers_file="$E2E_WORKDIR/headers.txt"
body_file="$E2E_WORKDIR/body.json"
status="$(e2e_curl "$body_file" "$headers_file" "${base_url}/v2/")"
[[ "$status" == "401" ]] || e2e_fail "expected 401 from /v2/, got ${status}"

api_version="$(e2e_header_value "$headers_file" 'Docker-Distribution-API-Version')"
[[ "$api_version" == 'registry/2.0' ]] || e2e_fail "unexpected API version header: ${api_version}"

challenge="$(e2e_header_value "$headers_file" 'WWW-Authenticate')"
[[ "$challenge" == Bearer* ]] || e2e_fail "expected bearer challenge, got: ${challenge}"

realm="$(e2e_bearer_param "$challenge" realm)"
service="$(e2e_bearer_param "$challenge" service)"
[[ -n "$realm" ]] || e2e_fail 'missing bearer realm'
[[ -n "$service" ]] || e2e_fail 'missing bearer service'

token="$(e2e_fetch_token "$realm" "$service" "$scope" "$E2E_PASSWORD")"
[[ -n "$token" && "$token" != 'null' ]] || e2e_fail 'token response did not include token'

payload_json="$(e2e_jwt_payload_json "$token")"
actions="$(e2e_token_actions "$payload_json" "$repo")"
[[ "$actions" == 'pull,push' ]] || e2e_fail "unexpected granted actions: ${actions:-<none>}"

bearer_headers="$E2E_WORKDIR/bearer-headers.txt"
bearer_body="$E2E_WORKDIR/bearer-body.json"
bearer_status="$(e2e_curl "$bearer_body" "$bearer_headers" --header "Authorization: Bearer ${token}" "${base_url}/v2/")"
[[ "$bearer_status" == '200' ]] || e2e_fail "expected bearer-authenticated /v2/ to return 200, got ${bearer_status}"

e2e_log "validated bearer challenge and token exchange against ${base_url}"
