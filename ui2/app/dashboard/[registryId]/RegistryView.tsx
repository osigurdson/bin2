'use client';

import Commands from "./Commands";

import { Registry } from '@/api/registries/types';
import Repositories from "./Repositories";
import { useGetRegistry } from "@/api/registries/hooks";

export default function RegistryView({ registryId }: { registryId: string }) {
  const { data: registry } = useGetRegistry(registryId);
  if (!registry) {
    return <div>Loading...</div>;
  }

  return (
    <>
      <div className="space-y-6">
        <div className="mb-4">Registry: <b>bin2.io/{registry.name}</b> (22 repos / 109.4 GiB)</div>
        <Commands id={registry.id} name={registry.name} apiKey="555" />
        <Repositories />
      </div>
    </>
  )
}
