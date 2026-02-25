import { Repository } from "@/api/repositories/types";

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

export default function Repositories() {
  const repos: Repository[] = [
    { id: '555', name: 'api', lastTag: '5', lastPush: new Date(2026, 2, 25) },
    { id: '556', name: 'stream', lastTag: '7', lastPush: new Date(2026, 2, 24) },
    { id: '557', name: 'py', lastTag: '3', lastPush: new Date(2026, 1, 22) },
  ];
  return (
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
            <td className="px-2 py-1">{repo.lastTag}</td>
            <td className="px-2 py-1">{formatTimeAgo(repo.lastPush)}</td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}
