#!/usr/bin/env bash
set -euo pipefail

E2E_TEST_NAME=pull_negative_paths
source "$(cd "$(dirname "$0")" && pwd)/lib.sh"

trap e2e_cleanup EXIT

e2e_require_core_env
e2e_require_cmd curl
e2e_require_cmd jq
e2e_setup_workdir

repo_leaf="$(e2e_unique_repo missing)"
repo="$(e2e_repo_path "$repo_leaf")"
missing_tag='missing'
missing_digest='sha256:0000000000000000000000000000000000000000000000000000000000000000'
bad_digest='sha256:not-a-real-digest'

challenge_headers="$E2E_WORKDIR/challenge-headers.txt"
challenge_body="$E2E_WORKDIR/challenge-body.txt"
challenge_status="$(e2e_curl "$challenge_body" "$challenge_headers" "$(e2e_pull_base_url)/v2/")"
[[ "$challenge_status" == '401' ]] || e2e_fail "expected pull /v2/ challenge to return 401, got ${challenge_status}"
challenge="$(e2e_header_value "$challenge_headers" 'WWW-Authenticate')"
realm="$(e2e_bearer_param "$challenge" realm)"
service="$(e2e_bearer_param "$challenge" service)"
token="$(e2e_fetch_token "$realm" "$service" "repository:${repo}:pull" "$E2E_PASSWORD")"

manifest_get_headers="$E2E_WORKDIR/manifest-get-headers.txt"
manifest_get_body="$E2E_WORKDIR/manifest-get-body.txt"
manifest_get_status="$(e2e_curl "$manifest_get_body" "$manifest_get_headers" \
  --header "Authorization: Bearer ${token}" \
  "$(e2e_pull_base_url)/v2/${repo}/manifests/${missing_tag}")"
[[ "$manifest_get_status" == '404' ]] || e2e_fail "expected missing manifest GET to return 404, got ${manifest_get_status}"

manifest_head_headers="$E2E_WORKDIR/manifest-head-headers.txt"
manifest_head_body="$E2E_WORKDIR/manifest-head-body.txt"
manifest_head_status="$(e2e_manifest_head "$(e2e_pull_base_url)" "$repo" "$missing_tag" "$token" "$manifest_head_headers" "$manifest_head_body")"
[[ "$manifest_head_status" == '404' ]] || e2e_fail "expected missing manifest HEAD to return 404, got ${manifest_head_status}"

blob_get_headers="$E2E_WORKDIR/blob-get-headers.txt"
blob_get_body="$E2E_WORKDIR/blob-get-body.json"
blob_get_status="$(e2e_curl "$blob_get_body" "$blob_get_headers" \
  --header "Authorization: Bearer ${token}" \
  "$(e2e_pull_base_url)/v2/${repo}/blobs/${missing_digest}")"
[[ "$blob_get_status" == '404' ]] || e2e_fail "expected missing blob GET to return 404, got ${blob_get_status}"
blob_get_code="$(jq -r '.errors[0].code // empty' "$blob_get_body")"
[[ "$blob_get_code" == 'BLOB_UNKNOWN' ]] || e2e_fail "expected missing blob GET code BLOB_UNKNOWN, got ${blob_get_code:-<none>}"

blob_head_headers="$E2E_WORKDIR/blob-head-headers.txt"
blob_head_body="$E2E_WORKDIR/blob-head-body.txt"
blob_head_status="$(e2e_curl "$blob_head_body" "$blob_head_headers" \
  --head \
  --header "Authorization: Bearer ${token}" \
  "$(e2e_pull_base_url)/v2/${repo}/blobs/${missing_digest}")"
[[ "$blob_head_status" == '404' ]] || e2e_fail "expected missing blob HEAD to return 404, got ${blob_head_status}"

bad_digest_headers="$E2E_WORKDIR/bad-digest-headers.txt"
bad_digest_body="$E2E_WORKDIR/bad-digest-body.json"
bad_digest_status="$(e2e_curl "$bad_digest_body" "$bad_digest_headers" \
  --header "Authorization: Bearer ${token}" \
  "$(e2e_pull_base_url)/v2/${repo}/blobs/${bad_digest}")"
[[ "$bad_digest_status" == '400' ]] || e2e_fail "expected invalid digest GET to return 400, got ${bad_digest_status}"
bad_digest_code="$(jq -r '.errors[0].code // empty' "$bad_digest_body")"
[[ "$bad_digest_code" == 'DIGEST_INVALID' ]] || e2e_fail "expected invalid digest code DIGEST_INVALID, got ${bad_digest_code:-<none>}"

e2e_log "validated pull-host negative manifest and blob paths for ${repo}"
