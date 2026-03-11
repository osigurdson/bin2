# OCI Distribution Conformance

Use [`run-oci-conformance.sh`](/home/owen/dev/bin2/app/tests/oci-conformance/run-oci-conformance.sh) to run the official `opencontainers/distribution-spec` conformance suite against the registry implementation.

## Prerequisites

- The API server is running.
- Postgres and object storage are configured the same way the server expects.

## Common Flows

Recommended: use an existing staging or production-like registry namespace and a real write-capable API key.
The default runner now executes the full OCI distribution suite.

```bash
./tests/oci-conformance/run-oci-conformance.sh \
  https://registry.example.com \
  myregistry/conformance-main
```

The runner will:

- default `OCI_USERNAME` to `bin2`
- prompt for `OCI_PASSWORD` when run interactively
- derive `OCI_CROSSMOUNT_NAMESPACE` automatically as a sibling namespace

## Notes

- The runner pins `opencontainers/distribution-spec` to `v1.1.1` by default. Override with `OCI_CONFORMANCE_REF` or `--ref`.
- The default run enables `pull`, `push`, `content discovery`, and `content management`.
- Results go to `test-results/oci-conformance` unless `OCI_REPORT_DIR` is set or `none`.
