import Link from "next/link";
import { getSignInUrl, withAuth } from "@workos-inc/authkit-nextjs";
import styles from "../page.module.css";
import { pricing, pricingDisplay } from "@/lib/pricing";

export const metadata = {
  title: "docs – bin2",
};

export default async function DocsPage() {
  const { user } = await withAuth();
  const signInUrl = await getSignInUrl();

  return (
    <div className={styles.container}>
      <header className={styles.header}>
        <Link className={styles.logo} href="/">
          bin<sub>2</sub>
        </Link>
        <nav className={styles.nav}>
          <a href="/#pricing" className={styles.navLink}>pricing</a>
          <Link href="/docs" className={`${styles.navLink} underline`}>docs</Link>
          {user ? (
            <Link href="/dashboard" className={styles.btn}>dashboard</Link>
          ) : (
            <a href={signInUrl} className={styles.btn}>login</a>
          )}
        </nav>
      </header>

      <div className="flex flex-col gap-10 py-10">

        {/* How it works */}
        <section className="flex flex-col gap-4">
          <h2 className="font-bold">How bin<sub>2</sub>.io works</h2>
          <p className="text-[#666]">
            bin<sub>2</sub>.io exposes two separate endpoints for push and pull traffic.
          </p>
          <div className="grid grid-cols-2 gap-4">
            <div className="border border-[#e8e8e8] p-5 flex flex-col gap-2">
              <p className="text-sm text-[#999]">Push endpoint</p>
              <p>bin<sub>2</sub>.io/&lt;registry&gt;</p>
              <p className="text-[#666]">
                Used for <code>docker push</code>, <code>podman push</code>, and ORAS uploads.
              </p>
            </div>
            <div className="border border-[#e8e8e8] p-5 flex flex-col gap-2">
              <p className="text-sm text-[#999]">Pull endpoint</p>
              <p>pull.bin<sub>2</sub>.io/&lt;registry&gt;</p>
              <p className="text-[#666]">
                Backed by a global CDN. Optimized for fast, low-cost downloads.
              </p>
            </div>
          </div>
          <p className="text-[#666]">
            While you can pull directly from <code>bin2.io</code>, it is slower and incurs higher costs.
            Use <code>pull.bin2.io</code> in CI, Kubernetes, and production environments.
          </p>
        </section>

        {/* Pricing */}
        <section className="flex flex-col gap-6">
          <div className="flex flex-col gap-2">
            <h2 className={styles.sectionTitle}>Pricing</h2>
            <p className="text-[#666]">
              bin<sub>2</sub>.io is designed to be a low-cost, commodity container registry.
              Pricing is based only on operations that incur real infrastructure costs.
            </p>
          </div>

          <div className="flex flex-col divide-y divide-[#e8e8e8]">

            {/* Push */}
            <div className="flex flex-col gap-2 py-6 first:pt-0">
              <h3 className="font-bold">Push Operations</h3>
              <p className="text-xl font-bold">${pricing.pushOpsPerMillion} <span className="text-sm font-normal text-[#999]">per million operations</span></p>
              <ul className="text-[#666] flex flex-col gap-1 mt-1">
                <li>– Docker images and ORAS artifacts consist of multiple layers.</li>
                <li>– Each pushed layer counts as one push operation.</li>
                <li>– Layers larger than 100 MiB incur one additional operation per 100 MiB.</li>
              </ul>
            </div>

            {/* Storage */}
            <div className="flex flex-col gap-2 py-6">
              <h3 className="font-bold">Storage</h3>
              <p className="text-xl font-bold">{pricingDisplay.storage}</p>
              <ul className="text-[#666] flex flex-col gap-1 mt-1">
                <li>– Billed in GiB-months using 30-day months.</li>
                <li>– Example: 10 GiB stored for half a month = 5 GiB-months = $0.10.</li>
              </ul>
            </div>

            {/* Pull */}
            <div className="flex flex-col gap-4 py-6">
              <h3 className="font-bold">Pull Operations</h3>
              <p className="text-[#666]">Pull costs depend on which endpoint is used.</p>
              <div className="grid grid-cols-2 gap-4">
                <div className="border border-[#e8e8e8] p-4 flex flex-col gap-2">
                  <p className="font-bold">CDN endpoint — pull.bin<sub>2</sub>.io</p>
                  <ul className="text-[#666] flex flex-col gap-1">
                    <li>– 1 pull operation per layer</li>
                    <li>– No egress fees</li>
                    <li>– {pricingDisplay.cdnPulls}</li>
                  </ul>
                </div>
                <div className="border border-[#e8e8e8] p-4 flex flex-col gap-2">
                  <p className="font-bold">Origin endpoint — bin<sub>2</sub>.io</p>
                  <ul className="text-[#666] flex flex-col gap-1">
                    <li>– 10 pull operations per layer</li>
                    <li>– $0.02 per GiB bandwidth</li>
                  </ul>
                </div>
              </div>
            </div>

            {/* Free tier */}
            <div className="flex flex-col gap-4 py-6">
              <h3 className="font-bold">Free Tier</h3>
              <div className="border border-[#111] p-5 flex flex-col gap-2">
                <p className="text-xl font-bold">{pricingDisplay.freeCredit} <span className="text-sm font-normal text-[#999]">free usage per month, every account</span></p>
                <p className="text-[#666]">
                  For hobbyists, home labs, and small startups, this will often cover typical usage entirely.
                </p>
              </div>
            </div>

          </div>
        </section>

        {/* Example */}
        <section className="flex flex-col gap-4">
          <h2 className="font-bold">Example: small team</h2>
          <p className="text-[#666]">
            10 CI builds per day, each producing 6 layers. 15 GiB stored. 300 CDN pulls per day.
          </p>
          <table className="w-full border-collapse">
            <thead>
              <tr className="border-b border-[#e8e8e8]">
                <th className="text-left py-2 pr-4 font-normal text-[#999]">Item</th>
                <th className="text-left py-2 pr-4 font-normal text-[#999]">Usage</th>
                <th className="text-right py-2 font-normal text-[#999]">Cost</th>
              </tr>
            </thead>
            <tbody>
              <tr className="border-b border-[#f0f0f0]">
                <td className="py-2 pr-4">Push operations</td>
                <td className="py-2 pr-4 text-[#666]">10 × 6 × 30 = 1,800 ops</td>
                <td className="py-2 text-right">$0.02</td>
              </tr>
              <tr className="border-b border-[#f0f0f0]">
                <td className="py-2 pr-4">Storage</td>
                <td className="py-2 pr-4 text-[#666]">15 GiB × 1 month</td>
                <td className="py-2 text-right">$0.30</td>
              </tr>
              <tr className="border-b border-[#e8e8e8]">
                <td className="py-2 pr-4">Pull operations (CDN)</td>
                <td className="py-2 pr-4 text-[#666]">300 × 6 × 30 = 54,000 ops</td>
                <td className="py-2 text-right">$0.11</td>
              </tr>
              <tr className="border-b border-[#f0f0f0]">
                <td className="py-2 pr-4 text-[#666]">Subtotal</td>
                <td></td>
                <td className="py-2 text-right text-[#666]">$0.43</td>
              </tr>
              <tr className="border-b border-[#f0f0f0]">
                <td className="py-2 pr-4 text-[#666]">Free tier credit</td>
                <td></td>
                <td className="py-2 text-right text-[#666]">−$1.00</td>
              </tr>
              <tr>
                <td className="py-2 pr-4 font-bold">Total</td>
                <td></td>
                <td className="py-2 text-right font-bold">$0.00</td>
              </tr>
            </tbody>
          </table>
        </section>

      </div>

      <footer className={styles.footer}>
        <p>
          bin<sub>2</sub> &copy; 2025 &middot; <a href="#">terms</a> &middot;{" "}
          <a href="#">privacy</a>
        </p>
      </footer>
    </div>
  );
}
