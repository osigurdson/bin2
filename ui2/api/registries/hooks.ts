import { useQuery } from '@tanstack/react-query';
import { Registry } from './types';

async function getRegistry(registryId: string): Promise<Registry> {
  const res = await fetch(`http://localhost:5000/api/v1/registries/${registryId}`);
  if (!res.ok) {
    throw new Error('Network response issue');
  }
  return res.json();
}

export function useGetRegistry(registryId: string) {
  return useQuery({
    queryKey: ['registry', registryId],
    queryFn: () => getRegistry(registryId),
    enabled: !!registryId,
  })
}
