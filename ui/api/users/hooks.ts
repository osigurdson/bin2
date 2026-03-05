import { useQuery } from '@tanstack/react-query';
import { useAccessToken } from '@workos-inc/authkit-nextjs/components';
import { apiV1Url } from '@/api/client';
import { CurrentUser } from './types';

export function useGetCurrentUser() {
  const { getAccessToken } = useAccessToken();

  return useQuery({
    queryKey: ['current-user'],
    queryFn: async () => {
      const token = await getAccessToken();
      const res = await fetch(apiV1Url('/users/me'), {
        headers: token ? { Authorization: `Bearer ${token}` } : {},
      });
      if (!res.ok) {
        throw new Error('Network response issue');
      }
      return res.json() as Promise<CurrentUser>;
    },
  });
}
