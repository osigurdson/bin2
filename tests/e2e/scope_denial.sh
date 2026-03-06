#!/usr/bin/env bash
set -euo pipefail

E2E_TEST_NAME=scope_denial
source "$(cd "$(dirname "$0")" && pwd)/lib.sh"

trap e2e_cleanup EXIT

e2e_require_core_env
e2e_require_cmd curl
e2e_require_cmd jq
e2e_require_cmd oras
e2e_require_cmd base64
e2e_setup_workdir

repo_leaf="$(e2e_unique_repo scope)"
repo="$(e2e_repo_path "$repo_leaf")"
tag='seed'
push_ref="$(e2e_push_ref "$repo_leaf" "$tag")"
artifact_file="$E2E_WORKDIR/scope.txt"
artifact_name="$(basename "$artifact_file")"

printf 'scope test %s\n' "$E2E_RUN_ID" >"$artifact_file"

seed_args=(--artifact-type application/vnd.bin2.e2e+txt -u "$E2E_REGISTRY_LOGIN_USER" -p "$E2E_PASSWORD")
if [[ "$E2E_PUSH_SCHEME" != 'https' ]]; then
  seed_args=(--plain-http "${seed_args[@]}")
fi

(
  cd "$E2E_WORKDIR"
  oras push "${seed_args[@]}" "$push_ref" "${artifact_name}:text/plain" >/dev/null
)

challenge_headers="$E2E_WORKDIR/challenge-headers.txt"
challenge_body="$E2E_WORKDIR/challenge-body.txt"
challenge_status="$(e2e_curl "$challenge_body" "$challenge_headers" "$(e2e_push_base_url)/v2/")"
[[ "$challenge_status" == '401' ]] || e2e_fail "expected push /v2/ challenge to return 401, got ${challenge_status}"
challenge="$(e2e_header_value "$challenge_headers" 'WWW-Authenticate')"
realm="$(e2e_bearer_param "$challenge" realm)"
service="$(e2e_bearer_param "$challenge" service)"

pull_token="$(e2e_fetch_token "$realm" "$service" "repository:${repo}:pull" "$E2E_PASSWORD")"
pull_payload="$(e2e_jwt_payload_json "$pull_token")"
pull_actions="$(e2e_token_actions "$pull_payload" "$repo")"
[[ "$pull_actions" == 'pull' ]] || e2e_fail "expected pull-only token to grant pull only, got ${pull_actions:-<none>}"

head_headers="$E2E_WORKDIR/head-headers.txt"
head_body="$E2E_WORKDIR/head-body.txt"
head_status="$(e2e_manifest_head "$(e2e_push_base_url)" "$repo" "$tag" "$pull_token" "$head_headers" "$head_body")"
[[ "$head_status" == '200' ]] || e2e_fail "expected pull-only manifest HEAD to succeed, got ${head_status}"

deny_headers="$E2E_WORKDIR/deny-headers.txt"
deny_body="$E2E_WORKDIR/deny-body.json"
deny_status="$(e2e_curl "$deny_body" "$deny_headers" --request POST --header "Authorization: Bearer ${pull_token}" "$(e2e_push_base_url)/v2/${repo}/blobs/uploads/")"
[[ "$deny_status" == '401' ]] || e2e_fail "expected pull-only upload start to be denied with 401, got ${deny_status}"
deny_code="$(jq -r '.errors[0].code // empty' "$deny_body")"
[[ "$deny_code" == 'DENIED' ]] || e2e_fail "expected DENIED OCI error code, got ${deny_code:-<none>}"

e2e_log "validated pull-only token scope enforcement for ${repo}"
