import { useQuery } from '@tanstack/react-query';
import { useAccessToken } from '@workos-inc/authkit-nextjs/components';
import { apiV1Url } from '@/api/client';
import { ListAPIKeysResponse } from './types';

export function useGetAPIKeys() {
  const { getAccessToken } = useAccessToken();

  return useQuery({
    queryKey: ['apikeys'],
    queryFn: async () => {
      const token = await getAccessToken();
      const res = await fetch(apiV1Url('/api-keys'), {
        headers: token ? { Authorization: `Bearer ${token}` } : {},
      });
      if (!res.ok) throw new Error('Failed to fetch API keys');
      return res.json() as Promise<ListAPIKeysResponse>;
    },
  });
}
