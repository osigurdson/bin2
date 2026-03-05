import { handleAuth } from '@workos-inc/authkit-nextjs';

function getAuthBaseURL(): string | undefined {
  const redirectUri = process.env.NEXT_PUBLIC_WORKOS_REDIRECT_URI?.trim();

  if (!redirectUri) {
    return undefined;
  }

  try {
    const url = new URL(redirectUri);
    return url.origin;
  } catch {
    return undefined;
  }
}

export const GET = handleAuth({
  returnPathname: '/dashboard',
  baseURL: getAuthBaseURL(),
});
