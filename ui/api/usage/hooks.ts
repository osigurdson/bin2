import { useQuery } from '@tanstack/react-query';
import { useAccessToken } from '@workos-inc/authkit-nextjs/components';
import { apiV1Url } from '@/api/client';
import { UsageSummary } from './types';

function currentUtcMonthWindow() {
  const now = new Date();
  const from = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), 1, 0, 0, 0, 0));
  const to = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth() + 1, 1, 0, 0, 0, 0));

  return {
    fromIso: from.toISOString().replace('.000Z', 'Z'),
    toIso: to.toISOString().replace('.000Z', 'Z'),
  };
}

export function useGetCurrentMonthUsageSummary() {
  const { getAccessToken } = useAccessToken();
  const { fromIso, toIso } = currentUtcMonthWindow();

  return useQuery({
    queryKey: ['usage-summary', fromIso, toIso],
    queryFn: async () => {
      const token = await getAccessToken();
      const params = new URLSearchParams({
        from: fromIso,
        to: toIso,
      });
      const res = await fetch(apiV1Url(`/usage/summary?${params.toString()}`), {
        cache: 'no-store',
        headers: token ? { Authorization: `Bearer ${token}` } : {},
      });
      if (!res.ok) {
        throw new Error('Network response issue');
      }
      return res.json() as Promise<UsageSummary>;
    },
    refetchInterval: 30000,
    refetchIntervalInBackground: true,
    staleTime: 0,
  });
}
