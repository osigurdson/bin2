import { withAuth } from '@workos-inc/authkit-nextjs';
import { Providers } from './providers';

export default async function Layout({ children }: { children: React.ReactNode }) {
  await withAuth({ ensureSignedIn: true });

  return (
    <Providers>
      {children}
    </Providers>
  );
}
