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
- Verifies Go-issued HS256 JWT using `REGISTRY_JWT_KEY`.
- Enforces JWT `aud` against `REGISTRY_SERVICE` (default `localhost:5000`).
- For manifest/blob endpoints, requires repository pull scope in `access`.
- Returns bearer challenge using
  `realm="http://localhost:5000/v2/token"` by default.

## Configuration

`wrangler.toml`:

- `REGISTRY_SERVICE`
- `REGISTRY_TOKEN_REALM`
- R2 bucket binding `BUCKET` (configured to `bin2`)

Worker secret:

```bash
wrangler secret put REGISTRY_JWT_KEY
```

## Run

```bash
npm install
npm run check
npm run dev
```
