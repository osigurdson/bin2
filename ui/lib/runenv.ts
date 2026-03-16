export function isDev(): boolean {
  const nodeEnv = process.env.NODE_ENV;
  if (nodeEnv === "production") {
    return false;
  } else if (nodeEnv === "development") {
    return true;
  }
  throw Error("process.env.NODE_ENV not defined")
}

export interface RegistryInfo {
  addr: string,
  pullAddr: string,
  isInsecure: boolean,
}

export function getRegistryInfo(): RegistryInfo {
  const addr = isDev() ? 'localhost:5000' : 'bin2.io';
  return {
    addr,
    pullAddr: isDev() ? addr : 'pull.bin2.io',
    isInsecure: isDev(),
  };
}

export function getSignoutRedirect(): string {
  return isDev() ? 'http://localhost:3000' : 'https://bin2.io';
}

export function getBackendApiOrigin(): string {
  return isDev() ? 'http://localhost:5000' : 'https://bin2.io';
}
