'use client';

import Commands from "./Commands";
import Repositories from "./Repositories";
import { useGetRegistry } from "@/api/registries/hooks";
import { useGetAPIKeys } from "@/api/apikeys/hooks";

export default function RegistryView({ registryId }: { registryId: string }) {
  const { data: registry } = useGetRegistry(registryId);
  const { data: keysData } = useGetAPIKeys();

  if (!registry) {
    return <div>Loading...</div>;
  }

  const registryKeys = keysData?.keys.filter(k =>
    k.scopes.some(s => s.registryId === registryId)
  ) ?? [];

  return (
    <>
      <div className="space-y-6">
        <div className="mb-4">
          <span>Registry: <b>bin2.io/{registry.name}</b> ({formatBytes(registry.sizeBytes)})</span>
        </div>
        <Commands id={registry.id} name={registry.name} apiKeys={registryKeys} />
        <Repositories registryId={registry.id} />
      </div>
    </>
  )
}

function formatBytes(bytes: number): string {
  const units = ["B", "KB", "MB", "GB", "TB"];
  let value = Number.isFinite(bytes) ? Math.max(0, bytes) : 0;
  let unitIndex = 0;

  while (value >= 1024 && unitIndex < units.length - 1) {
    value /= 1024;
    unitIndex += 1;
  }

  const precision = value >= 10 || unitIndex === 0 ? 0 : 1;
  return `${value.toFixed(precision)} ${units[unitIndex]}`;
}
