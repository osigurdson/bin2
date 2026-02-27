import Link from "next/link";
import { getSignInUrl, withAuth } from "@workos-inc/authkit-nextjs";

export default async function Home() {
  const { user } = await withAuth();
  const signInUrl = await getSignInUrl();

  if (user) {
    return (
      <div>
        <Link href="/dashboard">dashboard</Link>
      </div>
    );
  }

  return (
    <div>
      <a href={signInUrl}>Sign in</a>
    </div>
  );
}
