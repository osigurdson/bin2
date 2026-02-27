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
        <div className="mb-4">Registry: <b>bin2.io/{registry.name}</b> (22 repos / 109.4 GiB)</div>
        <Commands id={registry.id} name={registry.name} apiKeys={registryKeys} />
        <Repositories />
      </div>
    </>
  )
}
