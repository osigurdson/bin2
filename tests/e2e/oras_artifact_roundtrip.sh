#!/usr/bin/env bash
set -euo pipefail

E2E_TEST_NAME=oras_artifact_roundtrip
source "$(cd "$(dirname "$0")" && pwd)/lib.sh"

trap e2e_cleanup EXIT

e2e_require_core_env
e2e_require_cmd oras
e2e_setup_workdir

repo_leaf="$(e2e_unique_repo artifact)"
tag='v1'
push_ref="$(e2e_push_ref "$repo_leaf" "$tag")"
pull_ref="$(e2e_pull_ref "$repo_leaf" "$tag")"
artifact_file="$E2E_WORKDIR/hello.txt"
artifact_name="$(basename "$artifact_file")"
pull_dir="$E2E_WORKDIR/pull"
mkdir -p "$pull_dir"

printf 'hello from oras %s\n' "$E2E_RUN_ID" >"$artifact_file"

push_args=(--artifact-type application/vnd.bin2.e2e+txt -u "$E2E_REGISTRY_LOGIN_USER" -p "$E2E_PASSWORD")
pull_args=(-u "$E2E_REGISTRY_LOGIN_USER" -p "$E2E_PASSWORD" -o "$pull_dir")
if [[ "$E2E_PUSH_SCHEME" != 'https' ]]; then
  push_args=(--plain-http "${push_args[@]}")
fi
if [[ "$E2E_PULL_SCHEME" != 'https' ]]; then
  pull_args=(--plain-http "${pull_args[@]}")
fi

e2e_log "pushing ${push_ref} with oras"
(
  cd "$E2E_WORKDIR"
  oras push "${push_args[@]}" "$push_ref" "${artifact_name}:text/plain" >/dev/null
)

e2e_log "pulling ${pull_ref} with oras"
oras pull "${pull_args[@]}" "$pull_ref" >/dev/null

cmp -s "$artifact_file" "$pull_dir/hello.txt" || e2e_fail 'pulled artifact content did not match original'

e2e_log "validated oras artifact round-trip for ${E2E_NAMESPACE}/${repo_leaf}"
