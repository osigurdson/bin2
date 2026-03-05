import Link from "next/link";
import { SignedIn, SignedOut, SignInButton } from "@clerk/nextjs";

export default function Home() {
  return (
    <div className="container">
      <div className="above-fold">
        <header>
          <Link className="logo" href="/">
            bin<sub>2</sub>
          </Link>
          <nav>
            <a href="#pricing">pricing</a>
            <Link href="/docs">docs</Link>
            <SignedOut>
              <SignInButton mode="modal" forceRedirectUrl="/dashboard">
                <button className="btn">login</button>
              </SignInButton>
            </SignedOut>
            <SignedIn>
              <Link href="/dashboard" className="btn">
                dashboard
              </Link>
            </SignedIn>
          </nav>
        </header>

        <div className="hero">
          <h1>
            bin<sub>2</sub>
          </h1>
          <p className="tagline">
            the ridiculously cheap, fast and simple container registry
          </p>
          <a href="#pricing" className="cta">
            see pricing
          </a>
        </div>
      </div>

      <section id="pricing">
        <h2>Pricing</h2>
        <div className="tiers">
          <div className="tier">
            <h3>Free</h3>
            <div className="price">
              $0<span>/mo</span>
            </div>
            <ul>
              <li>1 GB storage</li>
              <li>Unlimited pulls</li>
              <li>1 private repo</li>
            </ul>
          </div>

          <div className="tier">
            <h3>Starter</h3>
            <div className="price">
              $5<span>/mo</span>
            </div>
            <ul>
              <li>100 GB storage</li>
              <li>Unlimited pulls</li>
              <li>Unlimited repos</li>
            </ul>
          </div>

          <div className="tier">
            <h3>Enterprise</h3>
            <div className="price">Call</div>
            <ul>
              <li>Unlimited storage</li>
              <li>Unlimited pulls</li>
              <li>Priority support</li>
              <li>Host in your own Cloudflare account</li>
            </ul>
          </div>
        </div>
      </section>

      <section id="pricing-explainer">
        <h2>Why is it the cheapest and the fastest?</h2>
        <p className="subtitle">
          Container storage ought to be a commodity. We treat it that way and
          remove the expensive parts of the pipeline and leverage Cloudflare&apos;s
          global CDN for incredible speed.
        </p>
        <div className="explainer">
          <div>
            <h3>Direct to R2</h3>
            <p>
              Standard registries write layers to registry storage first, then
              sync to object storage. bin2 streams layers straight to Cloudflare
              R2 during build, so there&apos;s no double-storage or internal
              bandwidth.
            </p>
          </div>
          <div>
            <h3>R2 economics</h3>
            <p>
              R2 is Cloudflare&apos;s hyper-competitive S3-equivalent storage, and
              Cloudflare doesn&apos;t charge for egress. With the data path built
              around R2 from day one, the cost floor is fundamentally lower than
              registries built on S3-style storage.
            </p>
          </div>
          <div>
            <h3>CLI-optimized push</h3>
            <p>
              Standard push clients expect the classic registry upload
              endpoints. Our CLI uses the R2-native push path and still gives
              you everything you need to build, tag, and manage images.
            </p>
          </div>
          <div>
            <h3>Your Cloudflare account</h3>
            <p>
              You can use our SaaS or host it yourself. We provide the logic,
              Cloudflare provides the infrastructure, and you own all the data
              in your own account with the same R2-native architecture.
            </p>
          </div>
        </div>
      </section>

      <footer>
        <p>
          bin2 &copy; 2025 &middot; <a href="#">terms</a> &middot;{" "}
          <a href="#">privacy</a>
        </p>
      </footer>
    </div>
  );
}
