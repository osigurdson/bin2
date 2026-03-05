import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useAccessToken } from '@workos-inc/authkit-nextjs/components';
import { apiV1Url } from '@/api/client';
import { ListRepositoriesResponse } from './types';

export class DeleteRepositoryError extends Error {
  status: number;

  constructor(status: number, message: string) {
    super(message);
    this.name = 'DeleteRepositoryError';
    this.status = status;
  }
}

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

export function useDeleteRepository(onSuccess?: (repositoryId: string, registryId: string) => void) {
  const queryClient = useQueryClient();
  const { getAccessToken } = useAccessToken();

  return useMutation({
    mutationFn: async ({ id, registryId }: { id: string; registryId: string }) => {
      const token = await getAccessToken();
      if (!token) {
        throw new DeleteRepositoryError(401, 'Missing access token');
      }

      const res = await fetch(
        apiV1Url(`/repositories/${encodeURIComponent(id)}?registryId=${encodeURIComponent(registryId)}`),
        {
          method: 'DELETE',
          headers: {
            Authorization: `Bearer ${token}`,
          },
        }
      );

      if (!res.ok) {
        throw new DeleteRepositoryError(res.status, `repository delete failed (${res.status})`);
      }

      return { id, registryId };
    },

    onSuccess: ({ id, registryId }) => {
      queryClient.setQueryData<ListRepositoriesResponse>(
        ['repositories', registryId],
        (previous) => {
          if (!previous) {
            return previous;
          }
          return {
            ...previous,
            repositories: previous.repositories.filter((r) => r.id !== id),
          };
        },
      );

      onSuccess?.(id, registryId);
    },
  });
}
