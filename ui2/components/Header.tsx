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
          <form action={() => signOut()}>
            <button type="submit">{user.email}</button>
          </form>
        )}
      </div>
    </div>
  );
}
