'use client';

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { useDeleteRegistry, useGetRegistries } from "@/api/registries/hooks";
import { useGetCurrentUser } from "@/api/users/hooks";
import RegistryCard from "./RegistryCard";
import MonthlyUsagePanel from "./MonthlyUsagePanel";
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
    if (isDeletePending) return;
    setRegistryPendingConfirm({ id: registryId, name: registryName });
  };

  const onCancelDeleteRegistry = () => {
    if (isDeletePending) return;
    setRegistryPendingConfirm(null);
  };

  const onConfirmDeleteRegistry = () => {
    if (!registryPendingConfirm) return;
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

  const registries = registriesData.registries;

  return (
    <div className="flex w-full flex-col gap-8">

      {/* Registries section */}
      <section className="min-h-56">
        <div className="mb-3 flex items-center justify-between">
          <div className="flex items-center gap-2">
            <span className="text-xs font-semibold uppercase tracking-widest opacity-40">Registries</span>
            {registries.length > 0 && (
              <span
                className="badge badge-ghost badge-sm opacity-60"
                aria-label={`${registries.length} ${registries.length === 1 ? 'registry' : 'registries'}`}
                title={`${registries.length} ${registries.length === 1 ? 'registry' : 'registries'}`}
              >
                {registries.length}
              </span>
            )}
          </div>
          <Link href="/dashboard/newRegistry" className="btn btn-sm btn-outline">
            + New registry
          </Link>
        </div>

        {registries.length === 0 ? (
          <div className="flex min-h-40 items-center justify-center rounded-xl border border-base-300 bg-base-100 px-4 py-6 text-center">
            <p className="text-sm opacity-60">No registries yet. Use the &quot;New registry&quot; button above to create one.</p>
          </div>
        ) : (
          <ul className="flex flex-col">
            {registries.map((registry) => (
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
        )}
      </section>

      {/* Usage section */}
      <div className="mt-4">
        <MonthlyUsagePanel />
      </div>

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
