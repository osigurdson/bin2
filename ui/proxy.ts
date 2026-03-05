import { authkitMiddleware } from '@workos-inc/authkit-nextjs';
import { type NextRequest, NextResponse, type NextFetchEvent } from 'next/server';

export default function middleware(request: NextRequest, event: NextFetchEvent) {
  // WorkOS redirects to /dashboard?code=... — rewrite internally to the auth handler
  if (request.nextUrl.pathname === '/dashboard' && request.nextUrl.searchParams.has('code')) {
    const url = request.nextUrl.clone();
    url.pathname = '/auth/callback';
    return NextResponse.rewrite(url);
  }
  return authkitMiddleware()(request, event);
}

export const config = { matcher: ['/((?!_next|.*\\..*).*)'] };
