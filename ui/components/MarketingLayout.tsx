import Link from "next/link";

const navLink = "text-inherit no-underline hover:underline";
const btn = "inline-flex items-center justify-center bg-base-content text-base-100 px-4 py-2 border border-base-content font-[inherit] leading-tight no-underline hover:bg-base-content/80 hover:border-base-content/80";

type MarketingLayoutProps = {
  children: React.ReactNode;
  user: { firstName?: string | null } | null;
  signInUrl: string;
  activeNav?: "pricing" | "docs";
};

export default function MarketingLayout({ children, user, signInUrl, activeNav }: MarketingLayoutProps) {
  const currentYear = new Date().getFullYear();

  return (
    <div className="max-w-3xl mx-auto px-5">
      <header className="flex justify-between items-center py-5 border-b border-base-200">
        <Link href="/" className="font-bold text-2xl no-underline text-inherit hover:no-underline">
          bin<sub>2</sub>
        </Link>
        <nav className="flex items-center gap-6">
          <Link href="/#pricing" className={`${navLink} ${activeNav === "pricing" ? "underline" : ""}`}>pricing</Link>
          <Link href="/docs" className={`${navLink} ${activeNav === "docs" ? "underline" : ""}`}>docs</Link>
          {user ? (
            <Link href="/dashboard" className={btn}>dashboard</Link>
          ) : (
            <a href={signInUrl} className={btn}>login</a>
          )}
        </nav>
      </header>

      {children}

      <footer className="border-t border-base-200 py-10 text-center text-base-content/40 text-sm">
        <p>
          bin<sub>2</sub> &copy; {currentYear} &middot; <Link href="/contact" className="text-base-content/60">contact</Link> &middot;{" "}
          <Link href="/terms" className="text-base-content/60">terms</Link> &middot;{" "}
          <Link href="/privacy" className="text-base-content/60">privacy</Link>
        </p>
      </footer>
    </div>
  );
}

export { navLink, btn };
