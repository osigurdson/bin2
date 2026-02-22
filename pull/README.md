# app/pull

Minimal Cloudflare Worker for OCI pull paths.

## Scope

Implemented endpoints:

- `GET|HEAD /v2`
- `GET|HEAD /v2/:repo/manifests/:reference`
- `GET|HEAD /v2/:repo/blobs/:digest`

Not implemented:

- `/v2/token` (served by Go API at `localhost:5000`)
- Push, upload, delete, tags, and catalog routes

## Auth model

- Requires `Authorization: Bearer <token>` on `/v2` routes.
- Verifies Go-issued EdDSA JWT using JWKS from Go API.
- Enforces JWT `aud` against `REGISTRY_SERVICE` (default `localhost:5000`).
- For manifest/blob endpoints, requires repository pull scope in `access`.
- Returns bearer challenge using
  `realm="http://localhost:5000/v2/token"` by default.

## Configuration

`wrangler.toml`:

- `REGISTRY_SERVICE`
- `REGISTRY_TOKEN_REALM`
- `REGISTRY_JWKS_URL`
- R2 bucket binding `BUCKET` (configured to `bin2`)

For local `wrangler dev`, keep `[[r2_buckets]].remote = true` so pull reads
the real R2 bucket instead of local emulated R2 state.

Generate an Ed25519 keypair for local use:

```bash
openssl genpkey -algorithm Ed25519 \
  -out /tmp/registry-private.pem
openssl pkey -in /tmp/registry-private.pem -pubout \
  -out /tmp/registry-public.pem
```

Set the Go API private key env:

```bash
export REGISTRY_JWT_PRIVATE_KEY_PEM="$(cat /tmp/registry-private.pem)"
```

If `REGISTRY_JWKS_URL` is omitted, pull derives it from token realm origin:

- `REGISTRY_TOKEN_REALM=http://localhost:5000/v2/token`
- derived JWKS URL: `http://localhost:5000/.well-known/jwks.json`

## Run

```bash
npm install
npm run check
npm run dev
```
