'use client';

import { Repository } from "@/api/repositories/types";
import { useDeleteRepository, useGetRepositories } from "@/api/repositories/hooks";
import ClipboardCopy from "@/components/ClipboardCopy";
import ConfirmModal from "@/components/ConfirmModal";
import { RefreshCw, Trash2 } from "lucide-react";
import { useState } from "react";
import type { ClientType } from "./Commands";

function formatTimeAgo(date: Date) {
  const diffMs = Date.now() - date.getTime();
  const diffMinutes = Math.floor(diffMs / (1000 * 60));

  if (diffMinutes < 60) {
    return `${Math.max(diffMinutes, 1)} minute${diffMinutes === 1 ? "" : "s"} ago`;
  }

  const diffHours = Math.floor(diffMinutes / 60);
  if (diffHours < 24) {
    return `${diffHours} hour${diffHours === 1 ? "" : "s"} ago`;
  }

  const diffDays = Math.floor(diffHours / 24);
  return `${diffDays} day${diffDays === 1 ? "" : "s"} ago`;
}

type RepositoriesProps = {
  registryId: string;
  registryName: string;
  selectedClient: ClientType;
};

export default function Repositories({ registryId, registryName, selectedClient }: RepositoriesProps) {
  const { data, isLoading, isError, refetch, isFetching } = useGetRepositories(registryId);
  const repos: Repository[] = data?.repositories ?? [];
  const [pendingDeleteRepo, setPendingDeleteRepo] = useState<Repository | null>(null);
  const [deletingRepositoryId, setDeletingRepositoryId] = useState<string | null>(null);
  const { mutate: deleteRepository, isPending: isDeletePending } = useDeleteRepository();
  const registryAddr = 'localhost:5000';
  const pushClient: Exclude<ClientType, 'k8s'> = selectedClient === 'k8s' ? 'docker' : selectedClient;
  const pullClient: 'docker' | 'podman' = selectedClient === 'podman' ? 'podman' : 'docker';
  const firstPullCommand = pullClient === 'podman'
    ? 'podman pull docker.io/library/hello-world:latest'
    : 'docker pull hello-world:latest';
  const firstPushTag = `${registryAddr}/${registryName}/hello-world:latest`;
  const firstPushCommand = pushClient === 'oras'
    ? `oras push ${firstPushTag} ./README.md:text/plain`
    : `${pushClient} push ${firstPushTag}`;
  const firstTagCommand = pushClient === 'oras'
    ? `oras tag ${firstPushTag} stable`
    : `${pushClient} tag hello-world:latest ${firstPushTag}`;
  const isRefreshing = isFetching && !isLoading;

  const onRequestDeleteRepository = (repo: Repository) => {
    if (isDeletePending) {
      return;
    }
    setPendingDeleteRepo(repo);
  };

  const onCancelDeleteRepository = () => {
    if (isDeletePending) {
      return;
    }
    setPendingDeleteRepo(null);
  };

  const onConfirmDeleteRepository = () => {
    if (!pendingDeleteRepo) {
      return;
    }
    const target = pendingDeleteRepo;
    setPendingDeleteRepo(null);
    setDeletingRepositoryId(target.id);
    deleteRepository(
      { id: target.id, registryId },
      {
        onError: () => {
          window.alert('Could not delete repository. Please try again.');
        },
        onSettled: () => {
          setDeletingRepositoryId(null);
        },
      },
    );
  };

  if (isLoading) {
    return (
      <div className="p-2">
        <RepositoryHeader onRefresh={() => refetch()} isRefreshing={isRefreshing} />
        <div className="text-sm opacity-70 mt-2">Loading repositories...</div>
      </div>
    );
  }

  if (isError) {
    return (
      <div className="p-2">
        <RepositoryHeader onRefresh={() => refetch()} isRefreshing={isRefreshing} />
        <div className="text-sm text-error mt-2">Could not load repositories.</div>
      </div>
    );
  }

  if (repos.length === 0) {
    return (
      <div className="p-2 space-y-3">
        <RepositoryHeader onRefresh={() => refetch()} isRefreshing={isRefreshing} />
        <div className="p-3">
          <p>No repositories yet. Push your first image after running the login command above. Some example
            tag and push commands below.</p>
          <div className="flex items-center gap-2 mt-2">
            <code className="">{firstPullCommand}</code>
            <ClipboardCopy copyText={firstPullCommand} />
          </div>
          <div className="">
            <div className="flex items-center gap-2">
              <code className="">{firstTagCommand}</code>
              <ClipboardCopy copyText={firstTagCommand} />
            </div>
            <div className="flex items-center gap-2">
              <code className="">{firstPushCommand}</code>
              <ClipboardCopy copyText={firstPushCommand} />
            </div>
          </div>
        </div>
        <ConfirmModal
          isOpen={!!pendingDeleteRepo}
          title="Delete Repository"
          message={pendingDeleteRepo
            ? `Delete repository ${pendingDeleteRepo.name}? This will remove tags and manifests associated with this repository.`
            : ''}
          confirmLabel="Delete"
          cancelLabel="Cancel"
          isConfirming={isDeletePending}
          confirmAction={onConfirmDeleteRepository}
          cancelAction={onCancelDeleteRepository}
        />
      </div>
    );
  }

  return (
    <div className="p-2">
      <RepositoryHeader onRefresh={() => refetch()} isRefreshing={isRefreshing} />
      <div className="p-1">
        <table className="w-full">
          <thead>
            <tr>
              <th className="py-1 text-left">Name</th>
              <th className="px-2 py-1 text-left">Last tag</th>
              <th className="px-2 py-1 text-left">Last push</th>
              <th className="px-2 py-1 text-right">Actions</th>
            </tr>
          </thead>
          <tbody>
            {repos.map((repo) => (
              <tr key={repo.id}>
                <td className="py-1">{repo.name}</td>
                <td className="px-2 py-1">{repo.lastTag ?? "-"}</td>
                <td className="px-2 py-1">{formatTimeAgo(new Date(repo.lastPush))}</td>
                <td className="px-2 py-1 text-right">
                  <button
                    type="button"
                    className="btn btn-ghost btn-xs btn-square text-error"
                    title={`Delete ${repo.name}`}
                    aria-label={`Delete ${repo.name}`}
                    disabled={isDeletePending && deletingRepositoryId === repo.id}
                    onClick={() => onRequestDeleteRepository(repo)}
                  >
                    {(isDeletePending && deletingRepositoryId === repo.id) ? (
                      <span className="loading loading-spinner loading-xs" aria-hidden />
                    ) : (
                      <Trash2 size={14} aria-hidden />
                    )}
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <ConfirmModal
        isOpen={!!pendingDeleteRepo}
        title="Delete Repository"
        message={pendingDeleteRepo
          ? `Delete repository ${pendingDeleteRepo.name}? This will remove tags and manifests associated with this repository.`
          : ''}
        confirmLabel="Delete"
        cancelLabel="Cancel"
        isConfirming={isDeletePending}
        confirmAction={onConfirmDeleteRepository}
        cancelAction={onCancelDeleteRepository}
      />
    </div>
  );
}

type RepositoryHeaderProps = {
  onRefresh: () => void;
  isRefreshing: boolean;
};

function RepositoryHeader({ onRefresh, isRefreshing }: RepositoryHeaderProps) {
  return (
    <div className="flex items-center justify-between gap-2">
      <span className="text-lg">Repositories</span>
      <button
        type="button"
        className="btn btn-ghost btn-xs btn-square"
        onClick={onRefresh}
        disabled={isRefreshing}
        title="Refresh repositories"
        aria-label="Refresh repositories"
      >
        <RefreshCw size={14} className={isRefreshing ? 'animate-spin' : ''} aria-hidden />
      </button>
    </div>
  );
}
