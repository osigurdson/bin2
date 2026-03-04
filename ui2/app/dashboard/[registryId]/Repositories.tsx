'use client';

import { Repository } from "@/api/repositories/types";
import { useDeleteRepository, useGetRepositories } from "@/api/repositories/hooks";
import ClipboardCopy from "@/components/ClipboardCopy";
import ConfirmModal from "@/components/ConfirmModal";
import { RefreshCw, Trash2 } from "lucide-react";
import { useState } from "react";
import type { ClientType } from "./Commands";
import { getRegistryInfo } from "@/lib/runenv";

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
  const { addr, isInsecure } = getRegistryInfo();
  const { data, isLoading, isError, refetch, isFetching } = useGetRepositories(registryId);
  const repos: Repository[] = data?.repositories ?? [];
  const [pendingDeleteRepo, setPendingDeleteRepo] = useState<Repository | null>(null);
  const [deletingRepositoryId, setDeletingRepositoryId] = useState<string | null>(null);
  const { mutate: deleteRepository, isPending: isDeletePending } = useDeleteRepository();
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

  const InstrComponent = instructions[selectedClient];

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
          <InstrComponent addr={addr} registry={registryName} isInsecure={isInsecure} />
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

// Header 
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

// Client specific instructions
interface InstructionProps {
  addr: string;
  registry: string;
  isInsecure: boolean;
}

type InstructionComponent = (props: InstructionProps) => React.ReactNode;

function DockerInstruction({ addr, registry, isInsecure }: InstructionProps) {
  const pullCmd = 'docker pull docker.io/library/hello-world:latest';
  const imageUri = `${addr}/${registry}/hello-world:latest`;
  const tagCmd = `docker tag docker.io/library/hello-world:latest ${imageUri}`;
  const pushCmd = `docker push ${imageUri}`;

  return (
    <div>No repositories yet. Push your first image after running the login
      command above. Some example tag and push commands below.

      {isInsecure && (
        <div className="text-warning opacity-70 mt-2">
          Note: Docker needs this registry configured as an insecure registry in daemon settings.
        </div>
      )}

      <div className="flex items-center gap-2 mt-2">
        <code>{pullCmd}</code>
        <ClipboardCopy copyText={pullCmd} />
      </div>
      <div className="flex items-center gap-2">
        <code>{tagCmd}</code>
        <ClipboardCopy copyText={tagCmd} />
      </div>
      <div className="flex items-center gap-2">
        <code>{pushCmd}</code>
        <ClipboardCopy copyText={pushCmd} />
      </div>
    </div>
  );
}

function PodmanInstruction({ addr, registry, isInsecure }: InstructionProps) {
  const pullCmd = 'podman pull docker.io/library/hello-world:latest';
  const imageUri = `${addr}/${registry}/hello-world:latest`;
  const tagCmd = `podman tag docker.io/library/hello-world:latest ${imageUri}`;
  const tlsVerify = isInsecure ? '--tls-verify=false' : '';
  const pushCmd = `podman push ${tlsVerify} ${imageUri}`;

  return (
    <div>No repositories yet. Push your first image after running the login
      command above. Some example tag and push commands below.

      <div className="flex items-center gap-2 mt-2">
        <code className="">{pullCmd}</code>
        <ClipboardCopy copyText={pullCmd} />
      </div>
      <div className="">
        <div className="flex items-center gap-2">
          <code className="">{tagCmd}</code>
          <ClipboardCopy copyText={tagCmd} />
        </div>
        <div className="flex items-center gap-2">
          <code className="">{pushCmd}</code>
          <ClipboardCopy copyText={pushCmd} />
        </div>
      </div>
    </div>
  );
}

function OrasInstruction({ addr, registry, isInsecure }: InstructionProps) {
  const artifactUri = `${addr}/${registry}/hello-artifact:v1`;
  const plainHttp = isInsecure ? '--plain-http ' : '';
  const createArtifactFileCmd = "echo 'hello from oras' > hello.txt";
  const pushCmd = `oras push ${plainHttp}${artifactUri} ./hello.txt:text/plain`;
  const pullCmd = `oras pull ${plainHttp}${artifactUri}`;

  return (
    <div>No repositories yet. Push your first OCI artifact after running the
      login command above. Some example push and pull commands below.

      <div className="flex items-center gap-2 mt-2">
        <code>{createArtifactFileCmd}</code>
        <ClipboardCopy copyText={createArtifactFileCmd} />
      </div>
      <div className="flex items-center gap-2">
        <code>{pushCmd}</code>
        <ClipboardCopy copyText={pushCmd} />
      </div>
      <div className="flex items-center gap-2">
        <code>{pullCmd}</code>
        <ClipboardCopy copyText={pullCmd} />
      </div>
    </div>
  );
}

function K8Instruction() {
  return (
    <div>No repositories yet. First push an image using docker or podman. Then
      create a pull secret in your cluster as shown above.
    </div>
  );
}

const instructions: Record<ClientType, InstructionComponent> = {
  docker: DockerInstruction,
  podman: PodmanInstruction,
  oras: OrasInstruction,
  k8s: K8Instruction,
}
