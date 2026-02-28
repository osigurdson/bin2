import {
  createRemoteJWKSet,
  jwtVerify,
  type JWTPayload,
} from "jose";

const apiVersion = "registry/2.0";
const defaultService = "localhost:5000";
const defaultTokenRealm = "http://localhost:5000/v2/token";
const defaultBlobType = "application/octet-stream";

const repoSegmentRe = /^[A-Za-z0-9._-]+$/;
const registryNameRe = /^[A-Za-z0-9_-]+$/;
const digestRe = /^sha256:([a-fA-F0-9]{64})$/;

type Env = {
  BUCKET: R2Bucket;
  REGISTRY_SERVICE?: string;
  REGISTRY_TOKEN_REALM?: string;
  REGISTRY_JWKS_URL?: string;
  REGISTRY_API_ORIGIN?: string;
};

type RegistryTokenAccess = {
  type: string;
  name: string;
  actions: string[];
};

type JWKSResolver = ReturnType<typeof createRemoteJWKSet>;

const jwksCache = new Map<string, JWKSResolver>();

export default {
  async fetch(request: Request, env: Env): Promise<Response> {
    const url = new URL(request.url);
    const method = request.method.toUpperCase();

    if (url.pathname === "/v2" || url.pathname === "/v2/") {
      return handleV2Root(request, env);
    }

    const manifestMatch = url.pathname.match(
      /^\/v2\/(.+)\/manifests\/([^/]+)$/,
    );
    if (manifestMatch !== null) {
      if (method !== "GET" && method !== "HEAD") {
        return ociError(
          method,
          404,
          "UNSUPPORTED",
          "endpoint not implemented",
        );
      }
      return handleManifest(request, env, manifestMatch[1], manifestMatch[2]);
    }

    const blobMatch = url.pathname.match(/^\/v2\/(.+)\/blobs\/([^/]+)$/);
    if (blobMatch !== null) {
      if (method !== "GET" && method !== "HEAD") {
        return ociError(
          method,
          404,
          "UNSUPPORTED",
          "endpoint not implemented",
        );
      }
      return handleBlob(request, env, blobMatch[1], blobMatch[2]);
    }

    return ociError(
      method,
      404,
      "UNSUPPORTED",
      "endpoint not implemented",
    );
  },
};

async function handleV2Root(request: Request, env: Env): Promise<Response> {
  const method = request.method.toUpperCase();
  if (method !== "GET" && method !== "HEAD") {
    return ociError(method, 405, "UNSUPPORTED", "method not allowed");
  }

  const auth = await authenticate(request, env, null);
  if (auth.response !== null) {
    return auth.response;
  }

  return new Response(null, {
    status: 200,
    headers: apiHeaders(),
  });
}

async function handleManifest(
  request: Request,
  env: Env,
  repo: string,
  reference: string,
): Promise<Response> {
  const method = request.method.toUpperCase();
  const auth = await authenticate(request, env, repo);
  if (auth.response !== null) {
    return auth.response;
  }

  if (!validRepoName(repo)) {
    return ociError(method, 400, "NAME_INVALID", "invalid repository name");
  }
  if (!validReference(reference)) {
    return ociError(
      method,
      400,
      "MANIFEST_INVALID",
      "invalid manifest reference",
    );
  }
  if (auth.namespace !== registryNamespace(repo)) {
    return ociError(
      method,
      403,
      "DENIED",
      "access denied to this repository",
    );
  }

  const reqURL = new URL(request.url);
  const upstreamOrigin = apiOrigin(env);
  if (reqURL.origin === upstreamOrigin) {
    return ociError(
      method,
      500,
      "UNKNOWN",
      "REGISTRY_API_ORIGIN must target the API origin, not the worker origin",
    );
  }

  const upstreamURL = new URL(`/v2/${repo}/manifests/${reference}`, upstreamOrigin);
  upstreamURL.search = reqURL.search;

  const forwardHeaders = new Headers();
  const authHeader = request.headers.get("Authorization");
  if (authHeader !== null) {
    forwardHeaders.set("Authorization", authHeader);
  }
  const accept = request.headers.get("Accept");
  if (accept !== null) {
    forwardHeaders.set("Accept", accept);
  }

  let upstreamResponse: Response;
  try {
    upstreamResponse = await fetch(upstreamURL.toString(), {
      method,
      headers: forwardHeaders,
      redirect: "manual",
    });
  } catch {
    return ociError(
      method,
      502,
      "UNKNOWN",
      "failed to load manifest",
    );
  }

  const headers = new Headers(upstreamResponse.headers);
  headers.set("Docker-Distribution-Api-Version", apiVersion);

  if (method === "HEAD") {
    return new Response(null, {
      status: upstreamResponse.status,
      headers,
    });
  }

  return new Response(upstreamResponse.body, {
    status: upstreamResponse.status,
    headers,
  });
}

async function handleBlob(
  request: Request,
  env: Env,
  repo: string,
  digest: string,
): Promise<Response> {
  const method = request.method.toUpperCase();
  const auth = await authenticate(request, env, repo);
  if (auth.response !== null) {
    return auth.response;
  }

  if (!validRepoName(repo)) {
    return ociError(method, 400, "NAME_INVALID", "invalid repository name");
  }
  const digestMatch = digest.match(digestRe);
  if (digestMatch === null) {
    return ociError(method, 400, "DIGEST_INVALID", "invalid digest");
  }
  if (auth.namespace !== registryNamespace(repo)) {
    return ociError(
      method,
      403,
      "DENIED",
      "access denied to this repository",
    );
  }

  const digestHex = digestMatch[1].toLowerCase();
  const key = blobObjectKey(digestHex);
  const object = await env.BUCKET.get(key);
  if (object === null) {
    return ociError(method, 404, "BLOB_UNKNOWN", "blob unknown");
  }

  const contentType = blobType(object.httpMetadata?.contentType);
  const headers = apiHeaders({
    "Content-Type": contentType,
    "Content-Length": String(object.size),
    "Docker-Content-Digest": `sha256:${digestHex}`,
  });

  if (method === "HEAD") {
    return new Response(null, {
      status: 200,
      headers,
    });
  }

  return new Response(object.body, {
    status: 200,
    headers,
  });
}

async function authenticate(
  request: Request,
  env: Env,
  repository: string | null,
): Promise<{ namespace: string; response: Response | null }> {
  const service = serviceName(env);
  const realm = tokenRealm(env);
  const scope = repository === null ? "" : formatRepositoryScope(repository);
  const auth = request.headers.get("Authorization")?.trim() ?? "";

  if (!auth.startsWith("Bearer ")) {
    return {
      namespace: "",
      response: unauthorizedResponse(
        request.method,
        realm,
        service,
        scope,
      ),
    };
  }

  const token = auth.slice("Bearer ".length).trim();
  if (token === "") {
    return {
      namespace: "",
      response: unauthorizedResponse(
        request.method,
        realm,
        service,
        scope,
      ),
    };
  }

  let jwks: JWKSResolver;
  try {
    jwks = loadJWKSResolver(env);
  } catch {
    return {
      namespace: "",
      response: ociError(
        request.method,
        500,
        "UNKNOWN",
        "invalid REGISTRY_JWKS_URL",
      ),
    };
  }

  let claims: JWTPayload;
  try {
    const verified = await jwtVerify(
      token,
      jwks,
      {
        algorithms: ["EdDSA"],
        audience: service,
        issuer: service,
        clockTolerance: "30s",
      },
    );
    claims = verified.payload;
  } catch {
    return {
      namespace: "",
      response: unauthorizedResponse(
        request.method,
        realm,
        service,
        scope,
      ),
    };
  }

  const namespace = (claims.sub ?? "").trim();
  if (namespace === "" || !validRegistryName(namespace)) {
    return {
      namespace: "",
      response: unauthorizedResponse(
        request.method,
        realm,
        service,
        scope,
      ),
    };
  }

  if (repository !== null && !tokenAllowsPull(claims, repository)) {
    return {
      namespace: "",
      response: deniedResponse(
        request.method,
        realm,
        service,
        scope,
      ),
    };
  }

  return {
    namespace,
    response: null,
  };
}

function tokenAllowsPull(claims: JWTPayload, repository: string): boolean {
  const accessRaw = claims.access;
  if (!Array.isArray(accessRaw)) {
    return false;
  }

  for (const entry of accessRaw) {
    if (!isRegistryTokenAccess(entry)) {
      continue;
    }
    if (entry.type !== "repository") {
      continue;
    }
    if (entry.name !== repository) {
      continue;
    }
    if (entry.actions.includes("pull") || entry.actions.includes("*")) {
      return true;
    }
  }

  return false;
}

function isRegistryTokenAccess(value: unknown): value is RegistryTokenAccess {
  if (typeof value !== "object" || value === null) {
    return false;
  }

  const item = value as Record<string, unknown>;
  if (typeof item.type !== "string" || typeof item.name !== "string") {
    return false;
  }
  if (!Array.isArray(item.actions)) {
    return false;
  }

  for (const action of item.actions) {
    if (typeof action !== "string") {
      return false;
    }
  }
  return true;
}

function unauthorizedResponse(
  method: string,
  realm: string,
  service: string,
  scope: string,
): Response {
  return ociError(
    method,
    401,
    "UNAUTHORIZED",
    "authentication required",
    {
      "Www-Authenticate": bearerChallenge(realm, service, scope),
    },
  );
}

function deniedResponse(
  method: string,
  realm: string,
  service: string,
  scope: string,
): Response {
  return ociError(
    method,
    401,
    "DENIED",
    "requested access to the resource is denied",
    {
      "Www-Authenticate": bearerChallenge(realm, service, scope),
    },
  );
}

function bearerChallenge(
  realm: string,
  service: string,
  scope: string,
): string {
  let challenge = `Bearer realm="${realm}",service="${service}"`;
  if (scope !== "") {
    challenge += `,scope="${scope}"`;
  }
  return challenge;
}

function ociError(
  method: string,
  status: number,
  code: string,
  message: string,
  headers?: HeadersInit,
): Response {
  const responseHeaders = apiHeaders({
    ...headers,
    "Content-Type": "application/json; charset=utf-8",
  });

  if (method.toUpperCase() === "HEAD") {
    return new Response(null, {
      status,
      headers: responseHeaders,
    });
  }

  return new Response(
    JSON.stringify({
      errors: [{ code, message }],
    }),
    {
      status,
      headers: responseHeaders,
    },
  );
}

function apiHeaders(init?: HeadersInit): Headers {
  const headers = new Headers(init);
  headers.set("Docker-Distribution-Api-Version", apiVersion);
  return headers;
}

function validRepoName(repo: string): boolean {
  if (repo === "" || repo.includes("..")) {
    return false;
  }

  const parts = repo.split("/");
  for (const part of parts) {
    if (part === "" || !repoSegmentRe.test(part)) {
      return false;
    }
  }
  return true;
}

function validReference(reference: string): boolean {
  if (reference === "") {
    return false;
  }
  if (reference.includes("/") || reference.includes("\\")) {
    return false;
  }
  return reference !== "." && reference !== "..";
}

function validRegistryName(name: string): boolean {
  if (name.length === 0 || name.length > 64) {
    return false;
  }
  return registryNameRe.test(name);
}

function registryNamespace(repo: string): string {
  const trimmed = repo.trim();
  if (trimmed === "") {
    return "";
  }
  const index = trimmed.indexOf("/");
  if (index === -1) {
    return trimmed;
  }
  return trimmed.slice(0, index);
}

function formatRepositoryScope(repo: string): string {
  return `repository:${repo}:pull`;
}

function blobObjectKey(digestHex: string): string {
  return `blobs/sha256/${digestHex.slice(0, 2)}/${digestHex}`;
}

function blobType(contentType: string | undefined): string {
  const trimmed = (contentType ?? "").trim();
  if (trimmed === "") {
    return defaultBlobType;
  }
  return trimmed;
}

function serviceName(env: Env): string {
  const service = (env.REGISTRY_SERVICE ?? "").trim();
  if (service !== "") {
    return service;
  }
  return defaultService;
}

function tokenRealm(env: Env): string {
  const realm = (env.REGISTRY_TOKEN_REALM ?? "").trim();
  if (realm !== "") {
    return realm;
  }
  return defaultTokenRealm;
}

function loadJWKSResolver(env: Env): JWKSResolver {
  const url = jwksURL(env);
  let resolver = jwksCache.get(url);
  if (resolver === undefined) {
    resolver = createRemoteJWKSet(new URL(url));
    jwksCache.set(url, resolver);
  }
  return resolver;
}

function jwksURL(env: Env): string {
  const explicit = (env.REGISTRY_JWKS_URL ?? "").trim();
  if (explicit !== "") {
    return explicit;
  }

  const realm = tokenRealm(env);
  return `${new URL(realm).origin}/.well-known/jwks.json`;
}

function apiOrigin(env: Env): string {
  const explicit = (env.REGISTRY_API_ORIGIN ?? "").trim();
  if (explicit !== "") {
    return new URL(explicit).origin;
  }
  return new URL(tokenRealm(env)).origin;
}
