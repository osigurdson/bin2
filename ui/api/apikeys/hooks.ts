import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useAccessToken } from '@workos-inc/authkit-nextjs/components';
import { apiV1Url } from '@/api/client';
import { ListAPIKeysResponse, APIKey } from './types';

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

export function useCreateAPIKey() {
  const { getAccessToken } = useAccessToken();
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (args: { keyName: string; scopes: { registryId: string; permission: string }[] }) => {
      const token = await getAccessToken();
      const res = await fetch(apiV1Url('/api-keys'), {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          ...(token ? { Authorization: `Bearer ${token}` } : {}),
        },
        body: JSON.stringify(args),
      });
      if (!res.ok) {
        const body = await res.json().catch(() => ({}));
        throw new Error(body.error ?? 'Failed to create API key');
      }
      return res.json() as Promise<APIKey>;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['apikeys'] });
    },
  });
}

export function useDeleteAPIKey() {
  const { getAccessToken } = useAccessToken();
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (id: string) => {
      const token = await getAccessToken();
      const res = await fetch(apiV1Url(`/api-keys/${id}`), {
        method: 'DELETE',
        headers: token ? { Authorization: `Bearer ${token}` } : {},
      });
      if (!res.ok && res.status !== 404) {
        throw new Error('Failed to delete API key');
      }
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['apikeys'] });
    },
  });
}
