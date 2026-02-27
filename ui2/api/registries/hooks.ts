import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useAccessToken } from '@workos-inc/authkit-nextjs/components';
import { apiV1Url } from '@/api/client';
import { ListRegistriesResponse, Registry } from './types';

export class CreateRegistryError extends Error {
  status: number;

  constructor(status: number, message: string) {
    super(message);
    this.name = 'CreateRegistryError';
    this.status = status;
  }
}

export function useGetRegistry(registryName: string) {
  const { getAccessToken } = useAccessToken();

  return useQuery({
    queryKey: ['registry', registryName],
    queryFn: async () => {
      const token = await getAccessToken();
      const res = await fetch(apiV1Url(`/registries/${encodeURIComponent(registryName)}`), {
        headers: token ? { Authorization: `Bearer ${token}` } : {},
      });
      if (!res.ok) {
        if (res.status === 404) {
          return null;
        }
        throw new Error('Network response issue');
      }
      return res.json() as Promise<Registry>;
    },
    enabled: !!registryName,
  });
}

async function fetchRegistryExists(registryName: string, token: string) {
  const url = apiV1Url(`/registries/exists?name=${registryName}`);
  const res = await fetch(
    url, {
    headers: {
      Authorization: `Bearer ${token}`,
    },
  });

  if (!res.ok) throw new Error('Request failed');
  return res.json();
}

export function useGetRegistryExists(registryName: string) {
  const { getAccessToken } = useAccessToken();

  return useQuery({
    queryKey: ['registry', registryName],
    queryFn: async () => {
      const token = await getAccessToken();
      if (!token) throw new Error('No access token');
      return fetchRegistryExists(registryName, token);
    },
    enabled: typeof registryName === 'string' && registryName.trim().length > 0,
  });
}

export function useGetRegistryByName(registryName: string) {
  return useGetRegistry(registryName);
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

export function useCreateRegistry(
  onSuccess?: (created: Registry) => void
) {
  const queryClient = useQueryClient();
  const { getAccessToken } = useAccessToken();

  return useMutation({
    mutationFn: async ({ name }: { name: string }) => {
      const token = await getAccessToken();
      if (!token) {
        throw new CreateRegistryError(401, 'Missing access token');
      }

      const res = await fetch(apiV1Url('/registries'), {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify({ name }),
      });

      if (!res.ok) {
        throw new CreateRegistryError(res.status, `registry create failed (${res.status})`);
      }

      return res.json() as Promise<Registry>;
    },

    onSuccess: (created) => {
      queryClient.setQueryData<ListRegistriesResponse>(
        ['registries'],
        (previous) => {
          const registries = previous?.registries ?? [];
          return {
            ...(previous ?? {}),
            registries: [created, ...registries],
          };
        }
      );

      onSuccess?.(created);
    },
  });
}
