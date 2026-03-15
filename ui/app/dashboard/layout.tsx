import { withAuth } from '@workos-inc/authkit-nextjs';
import { Providers } from './providers';
import Header from '@/components/Header';

export default async function Layout({ children }: { children: React.ReactNode }) {
  await withAuth({ ensureSignedIn: true });

  return (
    <Providers>
      <div className="flex flex-col min-h-dvh">

        {/* Header */}
        <div className="w-full">
          <div className="max-w-3xl mx-auto px-5">
            <div className="border-b border-base-200 py-5">
              <Header />
            </div>
          </div>
        </div>

        {/* Main */}
        <div className="flex-1 w-full">
          <div className="max-w-3xl mx-auto px-5 py-10">
            {children}
          </div>
        </div>

      </div>
    </Providers>
  );
}
