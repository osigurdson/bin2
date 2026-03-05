import { isDev } from '../lib/runenv';

function getBackendApiOrigin(): string {
  return isDev() ? 'http://localhost:5000' : 'https://bin2.nthesis.ai';
}

export function apiV1Url(path: string): string {
  const normalizedPath = path.startsWith('/') ? path : `/${path}`;
  return `${getBackendApiOrigin()}/api/v1${normalizedPath}`;
}
