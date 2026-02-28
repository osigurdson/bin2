'use client';

import Link from "next/link";
import Image, { type ImageLoaderProps } from "next/image";
import Logo from "./Logo";
import { useAuth } from "@workos-inc/authkit-nextjs/components";

const passthroughImageLoader = ({ src }: ImageLoaderProps) => src;

export default function Header() {
  const { user, signOut } = useAuth();
  const fallbackInitial = user?.firstName?.[0] ?? user?.email?.[0] ?? '?';

  return (
    <div className="flex justify-between items-center">
      <Link href="/dashboard" className="logo-mark"><Logo /></Link>
      <div>
        {user && (
          <div className="dropdown dropdown-end">
            <div
              tabIndex={0}
              role="button"
              aria-label="Open account menu"
              className="group -mt-px h-[30px] w-[30px] rounded-full border border-base-300 overflow-hidden transition-all duration-150 ease-out hover:-translate-y-px hover:border-base-content/20 hover:shadow-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-base-300"
            >
              {user.profilePictureUrl ? (
                <Image
                  loader={passthroughImageLoader}
                  unoptimized
                  src={user.profilePictureUrl}
                  width={30}
                  height={30}
                  alt={`${user.firstName ?? user.email} avatar`}
                  className="h-full w-full object-cover transition-transform duration-150 ease-out group-hover:scale-[1.03]"
                />
              ) : (
                <div
                  aria-hidden
                  className="h-full w-full flex items-center justify-center text-xs uppercase bg-base-200 transition-colors duration-150 ease-out group-hover:bg-base-300"
                >
                  {fallbackInitial}
                </div>
              )}
            </div>
            <ul
              tabIndex={0}
              className="dropdown-content menu z-20 mt-2 w-40 rounded-box border border-base-300 bg-base-100 p-2 shadow-sm"
            >
              <li>
                <button type="button" onClick={() => signOut()}>Sign out</button>
              </li>
            </ul>
          </div>
        )}
      </div>
    </div>
  );
}
