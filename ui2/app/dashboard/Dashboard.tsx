'use client';

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect } from "react";
import { useGetRegistries } from "@/api/registries/hooks";
import { useGetCurrentUser } from "@/api/users/hooks";
import RegistryCard from "./RegistryCard";
import { formatBytes } from "@/lib/formatBytes";

export default function Dashboard() {
  const router = useRouter();
  const {
    data: currentUser,
    isLoading: isLoadingCurrentUser,
    isError: isCurrentUserError,
  } = useGetCurrentUser();
  const {
    data: registriesData,
    isLoading: isLoadingRegistries,
    isError: isRegistriesError } = useGetRegistries();

  useEffect(() => {
    if (!isLoadingCurrentUser && currentUser && !currentUser.onboarded) {
      router.replace('/dashboard/newRegistry');
    }
  }, [currentUser, isLoadingCurrentUser, router]);

  if (isLoadingCurrentUser || isLoadingRegistries) {
    return <div className="p-2 text-sm opacity-70">Loading dashboard...</div>;
  }

  if (isCurrentUserError || isRegistriesError || !currentUser || !registriesData) {
    return <div className="p-2 text-sm text-error">Could not load registries.</div>;
  }

  if (!currentUser.onboarded) {
    return <div className="p-2 text-sm opacity-70">Preparing onboarding...</div>;
  }

  const totalSizeBytes = registriesData.registries.reduce((sum, registry) => {
    return sum + (registry.sizeBytes ?? 0);
  }, 0);

  if (registriesData.registries.length === 0) {
    return (
      <div className="max-w-xl rounded-lg border border-base-300 bg-base-100 p-6">
        <h2 className="text-xl font-bold">Create your first registry</h2>
        <p className="mt-2 text-sm opacity-75">
          Your dashboard is ready. Add a registry to start pushing and pulling container images.
        </p>
        <Link href="/dashboard/newRegistry" className="btn btn-primary mt-5">
          Create First Registry
        </Link>
      </div>
    );
  }

  return (
    <div className="flex flex-col max-w-xl">
      <div className="flex items-center justify-between gap-3">
        <div>
          {registriesData.registries.length} registries ({formatBytes(totalSizeBytes)})
        </div>
        <Link href="/dashboard/newRegistry" className="btn btn-sm btn-primary">
          Create Registry
        </Link>
      </div>
      <ul className="space-y-2 mt-4">
        {registriesData.registries.map((registry) => (
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
