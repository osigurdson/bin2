#!/usr/bin/env bash
set -euo pipefail

readonly TEST_ROOT="$(cd "$(dirname "$0")/e2e" && pwd)"

default_tests=(
  auth_handshake
  image_interop_roundtrip
  oras_artifact_roundtrip
  scope_denial
  index_manifest_validation
)

if [[ "$#" -gt 0 ]]; then
  tests=("$@")
else
  tests=("${default_tests[@]}")
fi

: "${E2E_RUN_ID:=$(date +%Y%m%d%H%M%S)-$$}"
export E2E_RUN_ID

status=0
for test_name in "${tests[@]}"; do
  script="${TEST_ROOT}/${test_name}.sh"
  if [[ ! -x "$script" ]]; then
    printf 'missing executable test: %s\n' "$script" >&2
    status=1
    continue
  fi

  printf '==> %s\n' "$test_name"
  if "$script"; then
    printf 'PASS %s\n' "$test_name"
  else
    printf 'FAIL %s\n' "$test_name" >&2
    status=1
  fi
  printf '\n'
done

exit "$status"
