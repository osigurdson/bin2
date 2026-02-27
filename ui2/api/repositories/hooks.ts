import { useQuery } from '@tanstack/react-query';
import { useAccessToken } from '@workos-inc/authkit-nextjs/components';
import { apiV1Url } from '@/api/client';
import { ListRepositoriesResponse } from './types';

export function useGetRepositories(registryId: string) {
  const { getAccessToken } = useAccessToken();

  return useQuery({
    queryKey: ['repositories', registryId],
    queryFn: async () => {
      const token = await getAccessToken();
      const res = await fetch(apiV1Url(`/repositories?registryId=${encodeURIComponent(registryId)}`), {
        headers: token ? { Authorization: `Bearer ${token}` } : {},
      });
      if (!res.ok) {
        throw new Error('Failed to fetch repositories');
      }
      return res.json() as Promise<ListRepositoriesResponse>;
    },
    enabled: !!registryId,
  });
}
