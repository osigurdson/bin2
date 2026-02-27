'use client';

import Link from "next/link";
import Logo from "./Logo";
import { useAuth } from "@workos-inc/authkit-nextjs/components";
import { signOut } from "@workos-inc/authkit-nextjs";

export default function Header() {
  const { user } = useAuth();

  return (
    <div className="flex justify-between items-center">
      <Link href="/dashboard" className="logo-mark"><Logo /></Link>
      <div>
        {user && (
          <div className="flex items-center gap-3">
            <span className="text-sm">{user.email}</span>
            <form action={() => signOut()}>
              <button type="submit" className="text-sm underline">Sign out</button>
            </form>
          </div>
        )}
      </div>
    </div>
  );
}
