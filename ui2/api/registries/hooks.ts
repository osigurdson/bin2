import { useQuery } from '@tanstack/react-query';
import { useAccessToken } from '@workos-inc/authkit-nextjs/components';
import { Registry } from './types';

export function useGetRegistry(registryId: string) {
  const { getAccessToken } = useAccessToken();

  return useQuery({
    queryKey: ['registry', registryId],
    queryFn: async () => {
      const token = await getAccessToken();
      const res = await fetch(`http://localhost:5000/api/v1/registries/${registryId}`, {
        headers: token ? { Authorization: `Bearer ${token}` } : {},
      });
      if (!res.ok) {
        throw new Error('Network response issue');
      }
      return res.json() as Promise<Registry>;
    },
    enabled: !!registryId,
  });
}
