import { getBackendApiOrigin } from '../lib/runenv';

export function apiV1Url(path: string): string {
  const normalizedPath = path.startsWith('/') ? path : `/${path}`;
  return `${getBackendApiOrigin()}/api/v1${normalizedPath}`;
}
