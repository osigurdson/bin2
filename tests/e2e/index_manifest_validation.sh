#!/usr/bin/env bash
set -euo pipefail

E2E_TEST_NAME=index_manifest_validation
source "$(cd "$(dirname "$0")" && pwd)/lib.sh"

trap e2e_cleanup EXIT

e2e_require_core_env
e2e_require_cmd curl
e2e_require_cmd jq
e2e_require_cmd oras
e2e_setup_workdir

repo_leaf="$(e2e_unique_repo index)"
repo="$(e2e_repo_path "$repo_leaf")"
child_tag='child-v1'
index_tag='index-v1'
invalid_tag='index-missing-child'
child_ref="$(e2e_push_ref "$repo_leaf" "$child_tag")"
artifact_file="$E2E_WORKDIR/index-child.txt"
artifact_name="$(basename "$artifact_file")"

printf 'index child %s\n' "$E2E_RUN_ID" >"$artifact_file"

oras_args=(--artifact-type application/vnd.bin2.e2e+txt -u "$E2E_REGISTRY_LOGIN_USER" -p "$E2E_PASSWORD")
if [[ "$E2E_PUSH_SCHEME" != 'https' ]]; then
  oras_args=(--plain-http "${oras_args[@]}")
fi

(
  cd "$E2E_WORKDIR"
  oras push "${oras_args[@]}" "$child_ref" "${artifact_name}:text/plain" >/dev/null
)

challenge_headers="$E2E_WORKDIR/challenge-headers.txt"
challenge_body="$E2E_WORKDIR/challenge-body.txt"
challenge_status="$(e2e_curl "$challenge_body" "$challenge_headers" "$(e2e_push_base_url)/v2/")"
[[ "$challenge_status" == '401' ]] || e2e_fail "expected push /v2/ challenge to return 401, got ${challenge_status}"
challenge="$(e2e_header_value "$challenge_headers" 'WWW-Authenticate')"
realm="$(e2e_bearer_param "$challenge" realm)"
service="$(e2e_bearer_param "$challenge" service)"
token="$(e2e_fetch_token "$realm" "$service" "repository:${repo}:pull,push" "$E2E_PASSWORD")"

child_head_headers="$E2E_WORKDIR/child-head-headers.txt"
child_head_body="$E2E_WORKDIR/child-head-body.txt"
child_head_status="$(e2e_manifest_head "$(e2e_push_base_url)" "$repo" "$child_tag" "$token" "$child_head_headers" "$child_head_body")"
[[ "$child_head_status" == '200' ]] || e2e_fail "expected child manifest HEAD to return 200, got ${child_head_status}"
child_digest="$(e2e_header_value "$child_head_headers" 'Docker-Content-Digest')"
child_media_type="$(e2e_header_value "$child_head_headers" 'Content-Type')"
child_size="$(e2e_header_value "$child_head_headers" 'Content-Length')"
[[ "$child_digest" == sha256:* ]] || e2e_fail "invalid child digest header: ${child_digest}"
[[ -n "$child_media_type" ]] || e2e_fail 'missing child content type header'
[[ "$child_size" =~ ^[0-9]+$ ]] || e2e_fail "invalid child size header: ${child_size}"

valid_index_json="$E2E_WORKDIR/valid-index.json"
invalid_index_json="$E2E_WORKDIR/invalid-index.json"
missing_digest='sha256:0000000000000000000000000000000000000000000000000000000000000000'

jq -n \
  --arg digest "$child_digest" \
  --arg mediaType "$child_media_type" \
  --argjson size "$child_size" \
  '{
    schemaVersion: 2,
    mediaType: "application/vnd.oci.image.index.v1+json",
    manifests: [
      {
        mediaType: $mediaType,
        digest: $digest,
        size: $size
      }
    ]
  }' >"$valid_index_json"

jq -n \
  --arg digest "$missing_digest" \
  --arg mediaType "$child_media_type" \
  --argjson size "$child_size" \
  '{
    schemaVersion: 2,
    mediaType: "application/vnd.oci.image.index.v1+json",
    manifests: [
      {
        mediaType: $mediaType,
        digest: $digest,
        size: $size
      }
    ]
  }' >"$invalid_index_json"

put_valid_headers="$E2E_WORKDIR/put-valid-headers.txt"
put_valid_body="$E2E_WORKDIR/put-valid-body.txt"
put_valid_status="$(e2e_curl "$put_valid_body" "$put_valid_headers" \
  --request PUT \
  --header "Authorization: Bearer ${token}" \
  --header 'Content-Type: application/vnd.oci.image.index.v1+json' \
  --data-binary "@${valid_index_json}" \
  "$(e2e_push_base_url)/v2/${repo}/manifests/${index_tag}")"
[[ "$put_valid_status" == '201' ]] || e2e_fail "expected valid index PUT to return 201, got ${put_valid_status}: $(cat "$put_valid_body")"
index_digest="$(e2e_header_value "$put_valid_headers" 'Docker-Content-Digest')"
[[ "$index_digest" == sha256:* ]] || e2e_fail "invalid index digest header: ${index_digest}"

index_head_headers="$E2E_WORKDIR/index-head-headers.txt"
index_head_body="$E2E_WORKDIR/index-head-body.txt"
index_head_status="$(e2e_manifest_head "$(e2e_push_base_url)" "$repo" "$index_tag" "$token" "$index_head_headers" "$index_head_body")"
[[ "$index_head_status" == '200' ]] || e2e_fail "expected index HEAD by tag to return 200, got ${index_head_status}"
index_tag_digest="$(e2e_header_value "$index_head_headers" 'Docker-Content-Digest')"
[[ "$index_tag_digest" == "$index_digest" ]] || e2e_fail "tag HEAD digest mismatch: put=${index_digest} head=${index_tag_digest}"

index_digest_headers="$E2E_WORKDIR/index-digest-headers.txt"
index_digest_body="$E2E_WORKDIR/index-digest-body.txt"
index_digest_status="$(e2e_manifest_head "$(e2e_push_base_url)" "$repo" "$index_digest" "$token" "$index_digest_headers" "$index_digest_body")"
[[ "$index_digest_status" == '200' ]] || e2e_fail "expected index HEAD by digest to return 200, got ${index_digest_status}"
index_digest_header="$(e2e_header_value "$index_digest_headers" 'Docker-Content-Digest')"
[[ "$index_digest_header" == "$index_digest" ]] || e2e_fail "digest HEAD mismatch: put=${index_digest} head=${index_digest_header}"

put_invalid_headers="$E2E_WORKDIR/put-invalid-headers.txt"
put_invalid_body="$E2E_WORKDIR/put-invalid-body.json"
put_invalid_status="$(e2e_curl "$put_invalid_body" "$put_invalid_headers" \
  --request PUT \
  --header "Authorization: Bearer ${token}" \
  --header 'Content-Type: application/vnd.oci.image.index.v1+json' \
  --data-binary "@${invalid_index_json}" \
  "$(e2e_push_base_url)/v2/${repo}/manifests/${invalid_tag}")"
[[ "$put_invalid_status" == '400' ]] || e2e_fail "expected invalid index PUT to return 400, got ${put_invalid_status}"
invalid_code="$(jq -r '.errors[0].code // empty' "$put_invalid_body")"
[[ "$invalid_code" == 'MANIFEST_BLOB_UNKNOWN' ]] || e2e_fail "expected MANIFEST_BLOB_UNKNOWN, got ${invalid_code:-<none>}"

e2e_log "validated OCI index acceptance and missing-child rejection for ${repo}"
