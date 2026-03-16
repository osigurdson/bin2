'use client';

import { Repository } from "@/api/repositories/types";
import { useDeleteRepository, useGetRepositories } from "@/api/repositories/hooks";
import ClipboardCopy from "@/components/ClipboardCopy";
import ConfirmModal from "@/components/ConfirmModal";
import { RefreshCw, Tag, Trash2 } from "lucide-react";
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
  const { addr, pullAddr, isInsecure } = getRegistryInfo();
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
      <div className="mt-8">
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
    <div className="mt-8">
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
            {repos.map((repo) => {
              const tag = repo.lastTag ?? 'latest';
              const pullCmd = buildPullCommand(selectedClient, pullAddr, registryName, repo.name, tag, isInsecure);
              return (
                <tr key={repo.id}>
                  <td className="py-1">{repo.name}</td>
                  <td className="px-2 py-1">
                    <div className="flex items-center justify-between gap-2">
                      <span>{repo.lastTag ?? <span className="opacity-40">-</span>}</span>
                      {repo.lastTag && (
                        <div className="flex items-center shrink-0 rounded bg-base-200/60 px-1">
                          <Tag size={11} className="opacity-40" />
                          <ClipboardCopy copyText={pullCmd} className="btn btn-ghost btn-xs btn-square text-base-content/45 hover:text-base-content/70" />
                        </div>
                      )}
                    </div>
                  </td>
                  <td className="pl-6 pr-2 py-1">{formatTimeAgo(new Date(repo.lastPush))}</td>
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
              );
            })}
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

function buildPullCommand(client: ClientType, pullAddr: string, registry: string, repo: string, tag: string, isInsecure: boolean): string {
  const imageUri = `${pullAddr}/${registry}/${repo}:${tag}`;
  switch (client) {
    case 'docker': return `docker pull ${imageUri}`;
    case 'podman': return `podman pull${isInsecure ? ' --tls-verify=false' : ''} ${imageUri}`;
    case 'oras': return `oras pull${isInsecure ? ' --plain-http' : ''} ${imageUri}`;
    case 'k8s': return `image: ${imageUri}`;
  }
}

// Client specific instructions
interface InstructionProps {
  addr: string;
  registry: string;
  isInsecure: boolean;
}

type InstructionComponent = (props: InstructionProps) => React.ReactNode;

function CombinedInstructions({ commands, note }: { commands: string[]; note?: React.ReactNode }) {
  const copyText = commands.join('\n');
  return (
    <div className="space-y-2">
      <p className="text-sm opacity-60">No repositories yet. Log in above, then paste these commands to push a test image.</p>
      {note}
      <div className="flex items-start gap-2">
        <pre className="text-xs flex-1">{commands.join('\n')}</pre>
        <ClipboardCopy copyText={copyText} />
      </div>
    </div>
  );
}

function DockerInstruction({ addr, registry, isInsecure }: InstructionProps) {
  const imageUri = `${addr}/${registry}/hello-world:latest`;
  const commands = [
    'docker pull hello-world:latest',
    `docker tag hello-world:latest ${imageUri}`,
    `docker push ${imageUri}`,
  ];
  const note = isInsecure ? (
    <p className="text-xs text-warning opacity-70">Note: Docker requires this registry configured as insecure in daemon settings.</p>
  ) : undefined;
  return <CombinedInstructions commands={commands} note={note} />;
}

function PodmanInstruction({ addr, registry, isInsecure }: InstructionProps) {
  const imageUri = `${addr}/${registry}/hello-world:latest`;
  const tlsVerify = isInsecure ? ' --tls-verify=false' : '';
  const commands = [
    'podman pull docker.io/library/hello-world:latest',
    `podman tag docker.io/library/hello-world:latest ${imageUri}`,
    `podman push${tlsVerify} ${imageUri}`,
  ];
  return <CombinedInstructions commands={commands} />;
}

function OrasInstruction({ addr, registry, isInsecure }: InstructionProps) {
  const artifactUri = `${addr}/${registry}/hello-artifact:v1`;
  const plainHttp = isInsecure ? '--plain-http ' : '';
  const commands = [
    "echo 'hello from oras' > hello.txt",
    `oras push ${plainHttp}${artifactUri} ./hello.txt:text/plain`,
    `oras pull ${plainHttp}${artifactUri}`,
  ];
  return <CombinedInstructions commands={commands} />;
}

function K8Instruction() {
  return (
    <p className="text-sm opacity-60">No repositories yet. Push an image using docker or podman first, then create a pull secret in your cluster using the manifest above.</p>
  );
}

const instructions: Record<ClientType, InstructionComponent> = {
  docker: DockerInstruction,
  podman: PodmanInstruction,
  oras: OrasInstruction,
  k8s: K8Instruction,
}
