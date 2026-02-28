'use client';

import Link from "next/link";
import { useGetRegistries } from "@/api/registries/hooks";
import RegistryCard from "./RegistryCard";
import { formatBytes } from "@/lib/formatBytes";

export default function Dashboard() {
  const { data, isLoading, isError } = useGetRegistries();

  if (isLoading) {
    return <div className="p-2 text-sm opacity-70">Loading registries...</div>;
  }

  if (isError || !data) {
    return <div className="p-2 text-sm text-error">Could not load registries.</div>;
  }

  const totalSizeBytes = data.registries.reduce((sum, registry) => {
    return sum + (registry.sizeBytes ?? 0);
  }, 0);

  if (data.registries.length === 0) {
    return (
      <div className="space-y-3">
        <p className="text-sm opacity-70">No registries yet.</p>
        <Link href="/dashboard/newRegistry" className="btn btn-sm">
          Create Registry
        </Link>
      </div>
    );
  }

  return (
    <div className="flex flex-col max-w-xl">
      <div className="">
        {data.registries.length} registries  ({formatBytes(totalSizeBytes)})
      </div>
      <ul className="space-y-2 mt-4">
        {data.registries.map((registry) => (
          <li key={registry.id}>
            <RegistryCard
              id={registry.id}
              name={registry.name}
              sizeBytes={registry.sizeBytes}
            />
          </li>
        ))}
      </ul>
    </div>
  );
}
