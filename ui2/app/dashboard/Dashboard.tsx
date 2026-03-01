'use client';

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { useDeleteRegistry, useGetRegistries } from "@/api/registries/hooks";
import { useGetCurrentUser } from "@/api/users/hooks";
import RegistryCard from "./RegistryCard";
import { formatBytes } from "@/lib/formatBytes";
import ConfirmModal from "@/components/ConfirmModal";

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
  const [deletingRegistryId, setDeletingRegistryId] = useState<string | null>(null);
  const [registryPendingConfirm, setRegistryPendingConfirm] = useState<{ id: string; name: string } | null>(null);
  const { mutate: deleteRegistry, isPending: isDeletePending } = useDeleteRegistry();

  useEffect(() => {
    if (!isLoadingCurrentUser && currentUser && !currentUser.onboarded) {
      router.replace('/dashboard/newRegistry');
    }
  }, [currentUser, isLoadingCurrentUser, router]);

  const onDeleteRegistry = (registryId: string, registryName: string) => {
    if (isDeletePending) {
      return;
    }
    setRegistryPendingConfirm({ id: registryId, name: registryName });
  };

  const onCancelDeleteRegistry = () => {
    if (isDeletePending) {
      return;
    }
    setRegistryPendingConfirm(null);
  };

  const onConfirmDeleteRegistry = () => {
    if (!registryPendingConfirm) {
      return;
    }
    const target = registryPendingConfirm;
    setRegistryPendingConfirm(null);
    setDeletingRegistryId(target.id);
    deleteRegistry(
      { id: target.id },
      {
        onError: () => {
          window.alert('Could not delete registry. Please try again.');
        },
        onSettled: () => {
          setDeletingRegistryId(null);
        },
      },
    );
  };

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
          Add a registry to start pushing and pulling container images.
        </p>
        <Link href="/dashboard/newRegistry" className="btn btn-primary mt-5">
          Create First Registry
        </Link>
      </div>
    );
  }

  let registriesHeaderText = '';
  if (registriesData.registries.length > 1) {
    registriesHeaderText =
      `${registriesData.registries.length} registries `;
    if (totalSizeBytes > 0) {
      registriesHeaderText = `${registriesHeaderText} (${formatBytes(totalSizeBytes)})`;
    }
  }
  return (
    <div className="flex flex-col max-w-xl">
      <div className="flex items-center justify-between gap-3">
        <div>
          {registriesHeaderText}
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
              onDelete={onDeleteRegistry}
              isDeleting={isDeletePending && deletingRegistryId === registry.id}
            />
          </li>
        ))}
      </ul>
      <ConfirmModal
        isOpen={!!registryPendingConfirm}
        title="Delete Registry"
        message={registryPendingConfirm
          ? `Delete bin2.io/${registryPendingConfirm.name}? This will permanently remove the registry and all associated repositories and tags.`
          : ''}
        confirmLabel="Delete"
        cancelLabel="Cancel"
        isConfirming={isDeletePending}
        confirmAction={onConfirmDeleteRegistry}
        cancelAction={onCancelDeleteRegistry}
      />
    </div>
  );
}
