#!/usr/bin/env bash
set -euo pipefail

E2E_TEST_NAME=blob_upload_protocol
source "$(cd "$(dirname "$0")" && pwd)/lib.sh"

trap e2e_cleanup EXIT

e2e_require_core_env
e2e_require_cmd curl
e2e_require_cmd jq
e2e_require_cmd sha256sum
e2e_setup_workdir

repo_leaf="$(e2e_unique_repo blob)"
repo="$(e2e_repo_path "$repo_leaf")"
blob_file="$E2E_WORKDIR/blob.txt"

printf 'blob upload protocol %s\n' "$E2E_RUN_ID" >"$blob_file"
blob_digest="sha256:$(sha256sum "$blob_file" | awk '{print $1}')"
blob_size="$(wc -c < "$blob_file" | tr -d '[:space:]')"
range_header="0-$(( blob_size - 1 ))"

challenge_headers="$E2E_WORKDIR/challenge-headers.txt"
challenge_body="$E2E_WORKDIR/challenge-body.txt"
challenge_status="$(e2e_curl "$challenge_body" "$challenge_headers" "$(e2e_push_base_url)/v2/")"
[[ "$challenge_status" == '401' ]] || e2e_fail "expected push /v2/ challenge to return 401, got ${challenge_status}"
challenge="$(e2e_header_value "$challenge_headers" 'WWW-Authenticate')"
realm="$(e2e_bearer_param "$challenge" realm)"
service="$(e2e_bearer_param "$challenge" service)"
token="$(e2e_fetch_token "$realm" "$service" "repository:${repo}:pull,push" "$E2E_PASSWORD")"

invalid_patch_headers="$E2E_WORKDIR/invalid-patch-headers.txt"
invalid_patch_body="$E2E_WORKDIR/invalid-patch-body.json"
invalid_patch_status="$(e2e_curl "$invalid_patch_body" "$invalid_patch_headers" \
  --request PATCH \
  --header "Authorization: Bearer ${token}" \
  --data-binary "@${blob_file}" \
  "$(e2e_push_base_url)/v2/${repo}/blobs/uploads/not-a-uuid")"
[[ "$invalid_patch_status" == '400' ]] || e2e_fail "expected invalid upload uuid PATCH to return 400, got ${invalid_patch_status}"
invalid_patch_code="$(jq -r '.errors[0].code // empty' "$invalid_patch_body")"
[[ "$invalid_patch_code" == 'BLOB_UPLOAD_INVALID' ]] || e2e_fail "expected BLOB_UPLOAD_INVALID, got ${invalid_patch_code:-<none>}"

unknown_patch_headers="$E2E_WORKDIR/unknown-patch-headers.txt"
unknown_patch_body="$E2E_WORKDIR/unknown-patch-body.json"
unknown_patch_status="$(e2e_curl "$unknown_patch_body" "$unknown_patch_headers" \
  --request PATCH \
  --header "Authorization: Bearer ${token}" \
  --data-binary "@${blob_file}" \
  "$(e2e_push_base_url)/v2/${repo}/blobs/uploads/00000000-0000-0000-0000-000000000000")"
[[ "$unknown_patch_status" == '404' ]] || e2e_fail "expected unknown upload uuid PATCH to return 404, got ${unknown_patch_status}"
unknown_patch_code="$(jq -r '.errors[0].code // empty' "$unknown_patch_body")"
[[ "$unknown_patch_code" == 'BLOB_UPLOAD_UNKNOWN' ]] || e2e_fail "expected BLOB_UPLOAD_UNKNOWN, got ${unknown_patch_code:-<none>}"

start_headers="$E2E_WORKDIR/start-headers.txt"
start_body="$E2E_WORKDIR/start-body.txt"
start_status="$(e2e_curl "$start_body" "$start_headers" \
  --request POST \
  --header "Authorization: Bearer ${token}" \
  "$(e2e_push_base_url)/v2/${repo}/blobs/uploads/")"
[[ "$start_status" == '202' ]] || e2e_fail "expected upload start to return 202, got ${start_status}"
upload_location="$(e2e_header_value "$start_headers" 'Location')"
[[ -n "$upload_location" ]] || e2e_fail 'missing upload location header'
upload_range="$(e2e_header_value "$start_headers" 'Range')"
[[ "$upload_range" == '0-0' ]] || e2e_fail "expected initial upload range 0-0, got ${upload_range}"
upload_url="$(e2e_resolve_location "$(e2e_push_base_url)" "$upload_location")"

patch_headers="$E2E_WORKDIR/patch-headers.txt"
patch_body="$E2E_WORKDIR/patch-body.txt"
patch_status="$(e2e_curl "$patch_body" "$patch_headers" \
  --request PATCH \
  --header "Authorization: Bearer ${token}" \
  --data-binary "@${blob_file}" \
  "$upload_url")"
[[ "$patch_status" == '202' ]] || e2e_fail "expected upload PATCH to return 202, got ${patch_status}"
patched_range="$(e2e_header_value "$patch_headers" 'Range')"
[[ "$patched_range" == "$range_header" ]] || e2e_fail "expected patched upload range ${range_header}, got ${patched_range}"

finalize_headers="$E2E_WORKDIR/finalize-headers.txt"
finalize_body="$E2E_WORKDIR/finalize-body.txt"
finalize_status="$(e2e_curl "$finalize_body" "$finalize_headers" \
  --request PUT \
  --header "Authorization: Bearer ${token}" \
  "${upload_url}?digest=${blob_digest}")"
[[ "$finalize_status" == '201' ]] || e2e_fail "expected upload finalize to return 201, got ${finalize_status}"
final_digest="$(e2e_header_value "$finalize_headers" 'Docker-Content-Digest')"
[[ "$final_digest" == "$blob_digest" ]] || e2e_fail "expected finalized digest ${blob_digest}, got ${final_digest}"

head_headers="$E2E_WORKDIR/head-headers.txt"
head_body="$E2E_WORKDIR/head-body.txt"
head_status="$(e2e_curl "$head_body" "$head_headers" \
  --head \
  --header "Authorization: Bearer ${token}" \
  "$(e2e_push_base_url)/v2/${repo}/blobs/${blob_digest}")"
[[ "$head_status" == '200' ]] || e2e_fail "expected blob HEAD to return 200, got ${head_status}"
head_digest="$(e2e_header_value "$head_headers" 'Docker-Content-Digest')"
[[ "$head_digest" == "$blob_digest" ]] || e2e_fail "expected blob HEAD digest ${blob_digest}, got ${head_digest}"
head_length="$(e2e_header_value "$head_headers" 'Content-Length')"
[[ "$head_length" == "$blob_size" ]] || e2e_fail "expected blob HEAD content length ${blob_size}, got ${head_length}"

get_headers="$E2E_WORKDIR/get-headers.txt"
get_body="$E2E_WORKDIR/get-body.bin"
get_status="$(e2e_curl "$get_body" "$get_headers" \
  --header "Authorization: Bearer ${token}" \
  "$(e2e_push_base_url)/v2/${repo}/blobs/${blob_digest}")"
[[ "$get_status" == '200' ]] || e2e_fail "expected blob GET to return 200, got ${get_status}"
cmp -s "$blob_file" "$get_body" || e2e_fail 'blob GET body did not match uploaded content'

put_only_headers="$E2E_WORKDIR/put-only-headers.txt"
put_only_body="$E2E_WORKDIR/put-only-body.txt"
put_only_start_status="$(e2e_curl "$put_only_body" "$put_only_headers" \
  --request POST \
  --header "Authorization: Bearer ${token}" \
  "$(e2e_push_base_url)/v2/${repo}/blobs/uploads/")"
[[ "$put_only_start_status" == '202' ]] || e2e_fail "expected second upload start to return 202, got ${put_only_start_status}"
put_only_url="$(e2e_resolve_location "$(e2e_push_base_url)" "$(e2e_header_value "$put_only_headers" 'Location')")"
put_only_finalize_headers="$E2E_WORKDIR/put-only-finalize-headers.txt"
put_only_finalize_body="$E2E_WORKDIR/put-only-finalize-body.txt"
put_only_finalize_status="$(e2e_curl "$put_only_finalize_body" "$put_only_finalize_headers" \
  --request PUT \
  --header "Authorization: Bearer ${token}" \
  --data-binary "@${blob_file}" \
  "${put_only_url}?digest=${blob_digest}")"
[[ "$put_only_finalize_status" == '201' ]] || e2e_fail "expected finalize-with-body upload to return 201, got ${put_only_finalize_status}"

mismatch_headers="$E2E_WORKDIR/mismatch-headers.txt"
mismatch_body="$E2E_WORKDIR/mismatch-body.json"
mismatch_start_status="$(e2e_curl "$mismatch_body" "$mismatch_headers" \
  --request POST \
  --header "Authorization: Bearer ${token}" \
  "$(e2e_push_base_url)/v2/${repo}/blobs/uploads/")"
[[ "$mismatch_start_status" == '202' ]] || e2e_fail "expected mismatch upload start to return 202, got ${mismatch_start_status}"
mismatch_url="$(e2e_resolve_location "$(e2e_push_base_url)" "$(e2e_header_value "$mismatch_headers" 'Location')")"
wrong_digest='sha256:0000000000000000000000000000000000000000000000000000000000000000'
mismatch_finalize_headers="$E2E_WORKDIR/mismatch-finalize-headers.txt"
mismatch_finalize_body="$E2E_WORKDIR/mismatch-finalize-body.json"
mismatch_finalize_status="$(e2e_curl "$mismatch_finalize_body" "$mismatch_finalize_headers" \
  --request PUT \
  --header "Authorization: Bearer ${token}" \
  --data-binary "@${blob_file}" \
  "${mismatch_url}?digest=${wrong_digest}")"
[[ "$mismatch_finalize_status" == '400' ]] || e2e_fail "expected digest mismatch upload to return 400, got ${mismatch_finalize_status}"
mismatch_code="$(jq -r '.errors[0].code // empty' "$mismatch_finalize_body")"
[[ "$mismatch_code" == 'DIGEST_INVALID' ]] || e2e_fail "expected DIGEST_INVALID, got ${mismatch_code:-<none>}"

e2e_log "validated direct blob upload protocol for ${repo}"
