#!/usr/bin/env bash
set -euo pipefail

E2E_TEST_NAME=image_interop_roundtrip
source "$(cd "$(dirname "$0")" && pwd)/lib.sh"

trap e2e_cleanup EXIT

e2e_require_core_env
e2e_require_cmd curl
e2e_require_cmd jq
e2e_require_cmd docker
e2e_require_cmd podman
e2e_setup_workdir

repo_leaf="$(e2e_unique_repo image)"
repo="$(e2e_repo_path "$repo_leaf")"
tag1='v1'
tag2='v2'
push_ref1="$(e2e_push_ref "$repo_leaf" "$tag1")"
pull_ref1="$(e2e_pull_ref "$repo_leaf" "$tag1")"
push_ref2="$(e2e_push_ref "$repo_leaf" "$tag2")"
pull_ref2="$(e2e_pull_ref "$repo_leaf" "$tag2")"

export DOCKER_CONFIG="$E2E_WORKDIR/docker-config"
mkdir -p "$DOCKER_CONFIG"
export REGISTRY_AUTH_FILE="$E2E_WORKDIR/podman-auth.json"

push_podman_args=()
pull_podman_args=()
if [[ "$E2E_PUSH_SCHEME" != 'https' ]]; then
  push_podman_args+=(--tls-verify=false)
fi
if [[ "$E2E_PULL_SCHEME" != 'https' ]]; then
  pull_podman_args+=(--tls-verify=false)
fi

printf '%s\n' "$E2E_PASSWORD" | docker login "$E2E_PUSH_REGISTRY" --username "$E2E_REGISTRY_LOGIN_USER" --password-stdin >/dev/null
printf '%s\n' "$E2E_PASSWORD" | podman login "${push_podman_args[@]}" "$E2E_PUSH_REGISTRY" --username "$E2E_REGISTRY_LOGIN_USER" --password-stdin >/dev/null
if [[ "$E2E_PULL_REGISTRY" != "$E2E_PUSH_REGISTRY" ]]; then
  printf '%s\n' "$E2E_PASSWORD" | docker login "$E2E_PULL_REGISTRY" --username "$E2E_REGISTRY_LOGIN_USER" --password-stdin >/dev/null
  printf '%s\n' "$E2E_PASSWORD" | podman login "${pull_podman_args[@]}" "$E2E_PULL_REGISTRY" --username "$E2E_REGISTRY_LOGIN_USER" --password-stdin >/dev/null
fi

e2e_log "pulling ${E2E_SOURCE_IMAGE} with docker"
docker pull "$E2E_SOURCE_IMAGE" >/dev/null

e2e_log "pushing ${push_ref1} with docker"
docker tag "$E2E_SOURCE_IMAGE" "$push_ref1"
docker push "$push_ref1" >/dev/null

challenge_headers="$E2E_WORKDIR/challenge-headers.txt"
challenge_body="$E2E_WORKDIR/challenge-body.json"
challenge_status="$(e2e_curl "$challenge_body" "$challenge_headers" "$(e2e_pull_base_url)/v2/")"
[[ "$challenge_status" == '401' ]] || e2e_fail "expected pull /v2/ challenge to return 401, got ${challenge_status}"
challenge="$(e2e_header_value "$challenge_headers" 'WWW-Authenticate')"
realm="$(e2e_bearer_param "$challenge" realm)"
service="$(e2e_bearer_param "$challenge" service)"
token="$(e2e_fetch_token "$realm" "$service" "repository:${repo}:pull" "$E2E_PASSWORD")"

head_tag_headers="$E2E_WORKDIR/head-tag-headers.txt"
head_tag_body="$E2E_WORKDIR/head-tag-body.txt"
head_tag_status="$(e2e_manifest_head "$(e2e_pull_base_url)" "$repo" "$tag1" "$token" "$head_tag_headers" "$head_tag_body")"
[[ "$head_tag_status" == '200' ]] || e2e_fail "expected manifest HEAD by tag to return 200, got ${head_tag_status}"
manifest_digest="$(e2e_header_value "$head_tag_headers" 'Docker-Content-Digest')"
[[ "$manifest_digest" == sha256:* ]] || e2e_fail "expected digest header from tag HEAD, got ${manifest_digest}"

e2e_log "pulling ${pull_ref1} with podman"
podman pull "${pull_podman_args[@]}" "$pull_ref1" >/dev/null

e2e_log "pulling ${manifest_digest} with docker"
docker pull "$(e2e_pull_digest_ref "$repo_leaf" "$manifest_digest")" >/dev/null

head_digest_headers="$E2E_WORKDIR/head-digest-headers.txt"
head_digest_body="$E2E_WORKDIR/head-digest-body.txt"
head_digest_status="$(e2e_manifest_head "$(e2e_pull_base_url)" "$repo" "$manifest_digest" "$token" "$head_digest_headers" "$head_digest_body")"
[[ "$head_digest_status" == '200' ]] || e2e_fail "expected manifest HEAD by digest to return 200, got ${head_digest_status}"
manifest_digest_by_ref="$(e2e_header_value "$head_digest_headers" 'Docker-Content-Digest')"
[[ "$manifest_digest_by_ref" == "$manifest_digest" ]] || e2e_fail "digest HEAD mismatch: tag=${manifest_digest} digest=${manifest_digest_by_ref}"

e2e_log "re-pushing ${push_ref2} with podman"
podman tag "$pull_ref1" "$push_ref2"
podman push "${push_podman_args[@]}" "$push_ref2" >/dev/null

e2e_log "pulling ${pull_ref2} with docker"
docker pull "$pull_ref2" >/dev/null

e2e_log "validated docker/podman interoperability for ${repo}"
