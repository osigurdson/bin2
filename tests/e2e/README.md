# Registry E2E Tests

These scripts exercise the registry through real client CLIs and direct HTTP calls.
They assume a pre-existing registry namespace plus at least one write-capable API key.

## Files

- `lib.sh`: shared helpers for registry URLs, token exchange, and API-key setup.
- `auth_handshake.sh`: bearer challenge and token exchange smoke test.
- `image_interop_roundtrip.sh`: Docker push, Podman pull/re-push, Docker digest pull.
- `oras_artifact_roundtrip.sh`: ORAS push and pull of a small file artifact.
- `scope_denial.sh`: a pull-only token can pull but cannot start uploads.
- `index_manifest_validation.sh`: valid OCI index is accepted; missing-child index is rejected.
- `../run-e2e.sh`: suite entrypoint.

## Required Environment

- `E2E_NAMESPACE`: registry namespace to test.
- `E2E_PASSWORD`: write-capable registry API key.

These must be exported environment variables, or provided inline for a single command.
Plain shell variables shown by `set` are not enough for `./tests/run-e2e.sh`.

## Common Optional Environment

- `E2E_PUSH_REGISTRY`: write registry host, default `localhost:5000`.
- `E2E_PULL_REGISTRY`: read registry host, defaults to `E2E_PUSH_REGISTRY`.
- `E2E_PUSH_SCHEME`: `http` or `https`, default `http`.
- `E2E_PULL_SCHEME`: `http` or `https`, defaults to `E2E_PUSH_SCHEME`.
- `E2E_SOURCE_IMAGE`: source image for the interop test, default `docker.io/library/hello-world:latest`.
- `E2E_RUN_ID`: repo/tag suffix shared across the suite. The runner sets one if omitted.

## Dependencies

Per script:

- `auth_handshake.sh`: `curl`, `jq`, `base64`
- `image_interop_roundtrip.sh`: `curl`, `jq`, `docker`, `podman`
- `oras_artifact_roundtrip.sh`: `oras`
- `scope_denial.sh`: `curl`, `jq`, `oras`, `base64`
- `index_manifest_validation.sh`: `curl`, `jq`, `oras`

## Examples

Run the full suite against a single local HTTP registry:

```bash
export E2E_NAMESPACE=nthesis
export E2E_PASSWORD=...
./tests/run-e2e.sh
```

Run only the ORAS and index tests:

```bash
E2E_NAMESPACE=nthesis \
E2E_PASSWORD=... \
./tests/run-e2e.sh oras_artifact_roundtrip index_manifest_validation
```

## Notes

- Client logins and token requests are hardcoded to username `bin2`.
- Docker HTTP testing still requires the daemon to trust the registry as insecure.
- The pull host may differ from the push host. The interop and auth tests support that split directly.
