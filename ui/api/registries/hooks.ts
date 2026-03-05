import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useAccessToken } from '@workos-inc/authkit-nextjs/components';
import { apiV1Url } from '@/api/client';
import { ListRegistriesResponse, Registry } from './types';
import { CurrentUser } from '@/api/users/types';

export class CreateRegistryError extends Error {
  status: number;

  constructor(status: number, message: string) {
    super(message);
    this.name = 'CreateRegistryError';
    this.status = status;
  }
}

export class DeleteRegistryError extends Error {
  status: number;

  constructor(status: number, message: string) {
    super(message);
    this.name = 'DeleteRegistryError';
    this.status = status;
  }
}

export function useGetRegistry(registryId: string) {
  const { getAccessToken } = useAccessToken();

  return useQuery({
    queryKey: ['registry', registryId],
    queryFn: async () => {
      const token = await getAccessToken();
      const res = await fetch(apiV1Url(`/registries/${encodeURIComponent(registryId)}`), {
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
    enabled: !!registryId,
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
      queryClient.setQueryData<CurrentUser>(
        ['current-user'],
        { onboarded: true },
      );

      onSuccess?.(created);
    },
  });
}

export function useDeleteRegistry(onSuccess?: (registryId: string) => void) {
  const queryClient = useQueryClient();
  const { getAccessToken } = useAccessToken();

  return useMutation({
    mutationFn: async ({ id }: { id: string }) => {
      const token = await getAccessToken();
      if (!token) {
        throw new DeleteRegistryError(401, 'Missing access token');
      }

      const res = await fetch(apiV1Url(`/registries/${encodeURIComponent(id)}`), {
        method: 'DELETE',
        headers: {
          Authorization: `Bearer ${token}`,
        },
      });

      if (!res.ok) {
        throw new DeleteRegistryError(res.status, `registry delete failed (${res.status})`);
      }

      return id;
    },

    onSuccess: (deletedRegistryId) => {
      queryClient.setQueryData<ListRegistriesResponse>(
        ['registries'],
        (previous) => {
          if (!previous) {
            return previous;
          }
          return {
            ...previous,
            registries: previous.registries.filter((r) => r.id !== deletedRegistryId),
          };
        }
      );

      onSuccess?.(deletedRegistryId);
    },
  });
}
