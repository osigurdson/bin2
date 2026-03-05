export interface APIKeyScope {
  registryId: string;
  repository: string | null;
  permission: 'read' | 'write' | 'admin';
  createdAt: string;
}

export interface APIKey {
  id: string;
  keyName: string;
  prefix: string;
  secretKey: string;
  createdAt: string;
  lastUsedAt: string | null;
  scopes: APIKeyScope[];
}

export interface ListAPIKeysResponse {
  keys: APIKey[];
}
