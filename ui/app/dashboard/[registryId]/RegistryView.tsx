'use client';

import Commands from "./Commands";
import Repositories from "./Repositories";
import { useGetRegistry } from "@/api/registries/hooks";
import { useGetAPIKeys } from "@/api/apikeys/hooks";
import { formatBytes } from "@/lib/formatBytes";
import { useState } from "react";
import type { ClientType } from "./Commands";

export default function RegistryView({ registryId }: { registryId: string }) {
  const { data: registry } = useGetRegistry(registryId);
  const { data: keysData } = useGetAPIKeys();
  const [selectedClient, setSelectedClient] = useState<ClientType>('docker');

  if (!registry) {
    return <div>Loading...</div>;
  }

  const registryKeys = keysData?.keys.filter(k =>
    k.scopes.some(s => s.registryId === registryId)
  ) ?? [];

  return (
    <div className="space-y-6">
      <div className="mb-4">
        <span><span className="text-lg">Registry: </span><b>bin2.io/{registry.name}</b> <RegistrySizeDisplay sizeBytes={registry.sizeBytes} /></span>
      </div>
      <Commands
        id={registry.id}
        name={registry.name}
        apiKeys={registryKeys}
        selectedClient={selectedClient}
        selectedClientChangeAction={setSelectedClient}
      />
      <Repositories registryId={registry.id} registryName={registry.name} selectedClient={selectedClient} />
    </div>
  );
}

function RegistrySizeDisplay({ sizeBytes }: { sizeBytes: number }) {
  if (sizeBytes === 0) {
    return null;
  }

  return <span>({formatBytes(sizeBytes)})</span>;
}
