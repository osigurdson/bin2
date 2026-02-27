const DEV_BACKEND_API_ORIGIN = 'http://localhost:5000';

function trimTrailingSlash(url: string): string {
  return url.replace(/\/+$/, '');
}

function getBackendApiOrigin(): string {
  const configuredOrigin = process.env.NEXT_PUBLIC_BACKEND_API_URL?.trim();
  if (configuredOrigin) {
    return trimTrailingSlash(configuredOrigin);
  }

  if (process.env.NODE_ENV !== 'production') {
    return DEV_BACKEND_API_ORIGIN;
  }

  throw new Error('Missing NEXT_PUBLIC_BACKEND_API_URL in production');
}

export function apiV1Url(path: string): string {
  const normalizedPath = path.startsWith('/') ? path : `/${path}`;
  return `${getBackendApiOrigin()}/api/v1${normalizedPath}`;
}
