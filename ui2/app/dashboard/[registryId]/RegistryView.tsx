'use client';

import Header from "@/components/Header";
import Commands from "./Commands";

import { Registry } from '@/api/registries/types';
import Repositories from "./Repositories";

export default function RegistryView({ registryId }: { registryId: string }) {
  const regInfo: Registry = {
    id: '555',
    name: 'nthesis',
  }
  return (
    <>
      <Header />
      <div className="w-full flex justify-center p-4">
        <div className="w-full max-w-xl text-left">
          <div className="mb-4"><b>bin2.io/{regInfo.name}</b> (22 repos / 109.4 GiB)</div>
          <Commands id={regInfo.id} name={regInfo.name} apiKey="555" />
          <Repositories />
        </div>
      </div>
    </>
  )
}
