import { getSignInUrl, withAuth } from "@workos-inc/authkit-nextjs";
import { pricing, pricingDisplay } from "@/lib/pricing";
import Bin2FlowDiagram from "@/components/Bin2FlowDiagram";
import MarketingLayout from "@/components/MarketingLayout";
import { Metadata } from 'next';

export const metadata: Metadata = {
  title: "docs – bin2",
};

export default async function DocsPage() {
  const { user } = await withAuth();
  const signInUrl = await getSignInUrl();

  return (
    <MarketingLayout user={user} signInUrl={signInUrl} activeNav="docs">
      <div className="flex flex-col gap-10 py-10">

        {/* How it works */}
        <section className="flex flex-col gap-4">
          <h2 className="font-bold">How bin<sub>2</sub>.io works</h2>
          <p className="text-base-content/60">
            bin<sub>2</sub>.io exposes two separate endpoints for push and
            pull operations. This reduces costs and improves performance by
            leveraging a global CDN when using the pull endpoint.
          </p>
          <Bin2FlowDiagram />
          <p className="text-base-content/60">
            While you can pull directly from <code>bin<sub>2</sub>.io</code>,
            it is slower and incurs higher costs. Use
            <code>pull.bin<sub>2</sub>.io</code> where possible.
          </p>
        </section>

        {/* Pricing */}
        <section className="flex flex-col gap-6">
          <div className="flex flex-col gap-2">
            <h2 className="text-xs uppercase tracking-[2px] text-base-content/40">Pricing</h2>
            <p className="text-base-content/60">
              bin<sub>2</sub>.io is designed to be a low-cost, commodity container registry.
              Pricing is based only on operations that incur real infrastructure costs.
            </p>
          </div>

          <div className="flex flex-col divide-y divide-base-200">

            {/* Push */}
            <div className="flex flex-col gap-2 py-6 first:pt-0">
              <h3 className="font-bold">Push Operations</h3>
              <p className="text-lg font-bold">${pricing.pushOpsPerMillion} <span className="text-sm font-normal text-base-content/40">per million operations</span></p>
              <ul className="text-sm text-base-content/60 flex flex-col gap-1 mt-1">
                <li>– Docker images and ORAS artifacts consist of multiple layers.</li>
                <li>– Each pushed layer counts as one push operation.</li>
                <li>– Layers larger than 100 MiB incur one additional operation per 100 MiB.</li>
              </ul>
            </div>

            {/* Storage */}
            <div className="flex flex-col gap-2 py-6">
              <h3 className="font-bold">Storage</h3>
              <p className="text-lg font-bold">{pricingDisplay.storage}</p>
              <ul className="text-sm text-base-content/60 flex flex-col gap-1 mt-1">
                <li>– Billed in GiB-months using 30-day months.</li>
                <li>– Example: 10 GiB stored for half a month = 5 GiB-months = $0.10.</li>
              </ul>
            </div>

            {/* Pull */}
            <div className="flex flex-col gap-4 py-6">
              <h3 className="font-bold">Pull Operations</h3>
              <p className="text-base-content/60">Pull costs depend on which endpoint is used.</p>
              <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
                <div className="border border-base-200 p-4 flex flex-col gap-2">
                  <p className="font-bold">CDN endpoint — pull.bin<sub>2</sub>.io</p>
                  <ul className="text-sm text-base-content/60 flex flex-col gap-1">
                    <li>– 1 pull operation per layer</li>
                    <li>– No egress fees</li>
                    <li>– {pricingDisplay.cdnPulls}</li>
                  </ul>
                </div>
                <div className="border border-base-200 p-4 flex flex-col gap-2">
                  <p className="font-bold">Origin endpoint — bin<sub>2</sub>.io</p>
                  <ul className="text-sm text-base-content/60 flex flex-col gap-1">
                    <li>– 10 pull operations per layer</li>
                    <li>– $0.02 per GiB bandwidth</li>
                  </ul>
                </div>
              </div>
            </div>

            {/* Free tier */}
            <div className="flex flex-col gap-4 py-6">
              <h3 className="font-bold">Free Tier</h3>
              <div className="border border-base-content p-5 flex flex-col gap-2">
                <p className="text-lg font-bold">{pricingDisplay.freeCredit} <span className="text-sm font-normal text-base-content/40">free usage per month, every account</span></p>
                <p className="text-base-content/60">
                  For hobbyists, home labs, and small startups, this will often cover typical usage entirely.
                </p>
              </div>
            </div>

          </div>
        </section>

        {/* Example */}
        <section className="flex flex-col gap-4">
          <h2 className="font-bold">Example: small team</h2>
          <p className="text-base-content/60">
            10 CI builds per day, each producing 6 layers. 15 GiB stored. 300 CDN pulls per day.
          </p>
          <table className="w-full border-collapse text-sm">
            <thead>
              <tr className="border-b border-base-200">
                <th className="text-left py-2 pr-4 font-normal text-base-content/40">Item</th>
                <th className="text-left py-2 pr-4 font-normal text-base-content/40">Usage</th>
                <th className="text-right py-2 font-normal text-base-content/40">Cost</th>
              </tr>
            </thead>
            <tbody>
              <tr className="border-b border-base-200">
                <td className="py-2 pr-4">Push operations</td>
                <td className="py-2 pr-4 text-base-content/60">10 × 6 × 30 = 1,800 ops</td>
                <td className="py-2 text-right">$0.02</td>
              </tr>
              <tr className="border-b border-base-200">
                <td className="py-2 pr-4">Storage</td>
                <td className="py-2 pr-4 text-base-content/60">15 GiB × 1 month</td>
                <td className="py-2 text-right">$0.30</td>
              </tr>
              <tr className="border-b border-base-200">
                <td className="py-2 pr-4">Pull operations (CDN)</td>
                <td className="py-2 pr-4 text-base-content/60">300 × 6 × 30 = 54,000 ops</td>
                <td className="py-2 text-right">$0.11</td>
              </tr>
              <tr className="border-b border-base-200">
                <td className="py-2 pr-4 text-base-content/60">Subtotal</td>
                <td></td>
                <td className="py-2 text-right text-base-content/60">$0.43</td>
              </tr>
              <tr className="border-b border-base-200">
                <td className="py-2 pr-4 text-base-content/60">Free tier credit</td>
                <td></td>
                <td className="py-2 text-right text-base-content/60">−$1.00</td>
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
    </MarketingLayout>
  );
}
