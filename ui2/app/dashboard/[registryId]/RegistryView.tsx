'use client';

import Commands from "./Commands";
import Repositories from "./Repositories";
import { useGetRegistry } from "@/api/registries/hooks";
import { useGetAPIKeys } from "@/api/apikeys/hooks";
import { formatBytes } from "@/lib/formatBytes";

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
