import { getSignInUrl, withAuth } from "@workos-inc/authkit-nextjs";
import { pricing, pricingDisplay } from "@/lib/pricing";
import MarketingLayout from "@/components/MarketingLayout";

export default async function Home() {
  const { user } = await withAuth();
  const signInUrl = await getSignInUrl();

  return (
    <MarketingLayout user={user} signInUrl={signInUrl}>
      {/* Hero — min-height pushes sections below the fold */}
      <div className="flex flex-col items-center justify-center text-center min-h-[calc(100vh-73px)] gap-8">
        <h1 className="text-5xl font-bold">bin<sub>2</sub></h1>
        <p className="text-lg text-[#666] max-w-md">
          the ridiculously cheap, fast and simple container registry
        </p>
        <a href="#pricing" className="inline-block bg-[#111] text-white px-8 py-3 no-underline hover:bg-[#333]">
          learn more
        </a>
      </div>

      {/* How it works */}
      <section className="py-[60px] border-t border-[#e8e8e8]">
        <h2 className="text-xs uppercase tracking-[2px] text-[#999] mb-8">How it works</h2>
        <p className="text-[#666] mb-8 max-w-[680px]">
          bin<sub>2</sub>.io separates push and pull traffic so each is served by the
          right infrastructure. All pull traffic is served by a global, low
          cost / high performance CDN.
        </p>
      </section>

      {/* Pricing */}
      <section id="pricing" className="py-[60px] border-t border-[#e8e8e8]">
        <h2 className="text-xs uppercase tracking-[2px] text-[#999] mb-8">Pricing</h2>
        <p className="text-[#666] mb-8 max-w-[680px]">
          Pay only for what you use. Every account receives {pricingDisplay.freeCredit} of
          free usage per month (which goes a long way - see example below).
          Docker images / ORAS artifacts consist of one or more layers. Pricing
          is based on layer operations and storage used.
        </p>
        <div className="grid grid-cols-3 gap-6">
          <div className="border border-[#e8e8e8] p-6">
            <h3 className="text-base font-semibold mb-2">Push ops</h3>
            <div className="text-3xl mb-4">
              ${pricing.pushOpsPerMillion}<span className="text-sm text-[#666]">/M ops</span>
            </div>
            <ul className="list-none p-0 text-sm text-[#666] flex flex-col gap-2">
              <li><span className="text-[#111]">+ </span>Per layer pushed</li>
              <li><span className="text-[#111]">+ </span>+1 op per {pricing.pushOpOverageMiBThreshold} MiB over {pricing.pushOpOverageMiBThreshold} MiB</li>
            </ul>
          </div>
          <div className="border border-[#e8e8e8] p-6">
            <h3 className="text-base font-semibold mb-2">Pull ops</h3>
            <div className="text-3xl mb-4">
              ${pricing.cdnPullOpsPerMillion}<span className="text-sm text-[#666]">/M ops</span>
            </div>
            <ul className="list-none p-0 text-sm text-[#666] flex flex-col gap-2">
              <li><span className="text-[#111]">+ </span>Per layer pulled</li>
              <li><span className="text-[#111]">+ </span>Via pull.bin<sub>2</sub>.io</li>
              <li><span className="text-[#111]">+ </span>No egress fees</li>
            </ul>
          </div>
          <div className="border border-[#e8e8e8] p-6">
            <h3 className="text-base font-semibold mb-2">Storage</h3>
            <div className="text-3xl mb-4">
              ${pricing.storagePerGiBMonth.toFixed(2)}<span className="text-sm text-[#666]">/GiB-mo</span>
            </div>
            <ul className="list-none p-0 text-sm text-[#666] flex flex-col gap-2">
              <li><span className="text-[#111]">+ </span>30-day months</li>
              <li><span className="text-[#111]">+ </span>Time-weighted billing</li>
            </ul>
          </div>
        </div>
      </section>

      {/* Example */}
      <section className="py-[60px] border-t border-[#e8e8e8]">
        <h2 className="text-xs uppercase tracking-[2px] text-[#999] mb-8">Example</h2>
        <p className="text-[#666] mb-8">
          10 CI builds per day, each producing 6 layers. 15 GiB stored. 300 CDN pulls per day.
        </p>
        <table className="w-full border-collapse text-sm">
          <thead>
            <tr>
              <th className="text-left px-3 py-2 border-b border-[#e8e8e8] text-[#999] font-normal">Item</th>
              <th className="text-left px-3 py-2 border-b border-[#e8e8e8] text-[#999] font-normal">Usage</th>
              <th className="text-right px-3 py-2 border-b border-[#e8e8e8] text-[#999] font-normal">Cost</th>
            </tr>
          </thead>
          <tbody>
            <tr>
              <td className="px-3 py-2 border-b border-[#f0f0f0]">Push operations</td>
              <td className="px-3 py-2 border-b border-[#f0f0f0]">10 × 6 × 30 = 1,800 ops</td>
              <td className="px-3 py-2 border-b border-[#f0f0f0] text-right">$0.02</td>
            </tr>
            <tr>
              <td className="px-3 py-2 border-b border-[#f0f0f0]">Storage</td>
              <td className="px-3 py-2 border-b border-[#f0f0f0]">15 GiB × 1 month</td>
              <td className="px-3 py-2 border-b border-[#f0f0f0] text-right">$0.30</td>
            </tr>
            <tr>
              <td className="px-3 py-2 border-b border-[#f0f0f0]">Pull operations (CDN)</td>
              <td className="px-3 py-2 border-b border-[#f0f0f0]">300 × 6 × 30 = 54,000 ops</td>
              <td className="px-3 py-2 border-b border-[#f0f0f0] text-right">$0.11</td>
            </tr>
            <tr>
              <td className="px-3 py-2 border-t border-[#e8e8e8] text-[#666]">Subtotal</td>
              <td className="px-3 py-2 border-t border-[#e8e8e8]"></td>
              <td className="px-3 py-2 border-t border-[#e8e8e8] text-right text-[#666]">$0.43</td>
            </tr>
            <tr>
              <td className="px-3 py-2 text-[#666]">Free tier credit</td>
              <td className="px-3 py-2"></td>
              <td className="px-3 py-2 text-right text-[#666]">−{pricingDisplay.freeCredit}</td>
            </tr>
            <tr>
              <td className="px-3 py-2 font-bold">Total</td>
              <td className="px-3 py-2"></td>
              <td className="px-3 py-2 text-right font-bold">$0.00</td>
            </tr>
          </tbody>
        </table>
      </section>
    </MarketingLayout>
  );
}
