'use client';

import { Repository } from "@/api/repositories/types";
import { useGetRepositories } from "@/api/repositories/hooks";

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
};

export default function Repositories({ registryId }: RepositoriesProps) {
  const { data, isLoading, isError } = useGetRepositories(registryId);
  const repos: Repository[] = data?.repositories ?? [];

  if (isLoading) {
    return <div className="p-2 text-sm opacity-70">Loading repositories...</div>;
  }

  if (isError) {
    return <div className="p-2 text-sm text-error">Could not load repositories.</div>;
  }

  if (repos.length === 0) {
    return null;
  }
  return (
    <div className="p-2">
      <table className="w-full">
        <thead>
          <tr>
            <th className="py-1 text-left">Repository</th>
            <th className="px-2 py-1 text-left">Last tag</th>
            <th className="px-2 py-1 text-left">Last push</th>
          </tr>
        </thead>
        <tbody>
          {repos.map((repo) => (
            <tr key={repo.id}>
              <td className="py-1">{repo.name}</td>
              <td className="px-2 py-1">{repo.lastTag ?? "-"}</td>
              <td className="px-2 py-1">{formatTimeAgo(new Date(repo.lastPush))}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
