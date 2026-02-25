import { useQuery } from '@tanstack/react-query';
import { useAccessToken } from '@workos-inc/authkit-nextjs/components';
import { apiV1Url } from '@/api/client';
import { ListRegistriesResponse, Registry } from './types';

export function useGetRegistry(registryId: string) {
  const { getAccessToken } = useAccessToken();

  return useQuery({
    queryKey: ['registry', registryId],
    queryFn: async () => {
      const token = await getAccessToken();
      const res = await fetch(apiV1Url(`/registries/${registryId}`), {
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

export function useGetRegistries() {
  const { getAccessToken } = useAccessToken();

  return useQuery({
    queryKey: ['registries'],
    queryFn: async () => {
      const token = await getAccessToken();
      const res = await fetch(apiV1Url('/registries'), {
        headers: token ? { Authorization: `Bearer ${token}` } : {},
      });
      if (!res.ok) {
        throw new Error('Network response issue')
      }
      return res.json() as Promise<ListRegistriesResponse>;
    }
  })
}
