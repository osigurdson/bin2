'use client';

import Link from "next/link";
import Logo from "./Logo";

export default function Header() {
  return (
    <div className="flex justify-between items-center p-2 shadow-sm">
      <Link href="/dashboard" className="logo-mark"><Logo /></Link>
      <div>userbutton</div>
    </div>
  );
}
