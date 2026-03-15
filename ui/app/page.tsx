import Link from "next/link";
import { getSignInUrl, withAuth } from "@workos-inc/authkit-nextjs";
import styles from "./page.module.css";
import { pricing, pricingDisplay } from "@/lib/pricing";

export default async function Home() {
  const { user } = await withAuth();
  const signInUrl = await getSignInUrl();

  return (
    <div className={styles.container}>
      <div className={styles.aboveFold}>
        <header className={styles.header}>
          <Link className={styles.logo} href="/">
            bin<sub>2</sub>
          </Link>
          <nav className={styles.nav}>
            <a href="#pricing" className={styles.navLink}>pricing</a>
            <Link href="/docs" className={styles.navLink}>docs</Link>
            {user ? (
              <Link href="/dashboard" className={styles.btn}>dashboard</Link>
            ) : (
              <a href={signInUrl} className={styles.btn}>login</a>
            )}
          </nav>
        </header>

        <div className={styles.hero}>
          <h1>bin<sub>2</sub></h1>
          <p className={styles.tagline}>
            the ridiculously cheap, fast and simple container registry
          </p>
          <a href="#pricing" className={styles.cta}>see pricing</a>
        </div>
      </div>

      {/* How it works */}
      <section className={styles.section}>
        <h2 className={styles.sectionTitle}>How it works</h2>
        <p className={styles.subtitle}>
          bin<sub>2</sub>.io separates push and pull traffic so each is served by the
          right infrastructure. All pull traffic is served by a global, low
          cost / high performance CDN.
        </p>
      </section>

      {/* Pricing */}
      <section id="pricing" className={styles.section}>
        <h2 className={styles.sectionTitle}>Pricing</h2>
        <p className={styles.subtitle}>
          Pay only for what you use. Every account receives {pricingDisplay.freeCredit} of
          free usage per month. Docker images / ORAS artifacts consist of one
          or more layers. Pricing is based on layer operations and storage used.
        </p>
        <div className={styles.tiers}>
          <div className={styles.tier}>
            <h3>Push ops</h3>
            <div className={styles.price}>
              ${pricing.pushOpsPerMillion}<span>/M ops</span>
            </div>
            <ul>
              <li>Per layer pushed</li>
              <li>+1 op per {pricing.pushOpOverageMiBThreshold} MiB over {pricing.pushOpOverageMiBThreshold} MiB</li>
            </ul>
          </div>
          <div className={styles.tier}>
            <h3>Pull ops</h3>
            <div className={styles.price}>
              ${pricing.cdnPullOpsPerMillion}<span>/M ops</span>
            </div>
            <ul>
              <li>Per layer pulled</li>
              <li>Via pull.bin<sub>2</sub>.io</li>
              <li>No egress fees</li>
            </ul>
          </div>
          <div className={styles.tier}>
            <h3>Storage</h3>
            <div className={styles.price}>
              ${pricing.storagePerGiBMonth.toFixed(2)}<span>/GiB-mo</span>
            </div>
            <ul>
              <li>30-day months</li>
              <li>Time-weighted billing</li>
            </ul>
          </div>
        </div>
      </section>

      {/* Example */}
      <section className={styles.section}>
        <h2 className={styles.sectionTitle}>Example</h2>
        <p className={styles.subtitle}>
          10 CI builds per day, each producing 6 layers. 15 GiB stored. 300 CDN pulls per day.
        </p>
        <table className={styles.exampleTable}>
          <thead>
            <tr>
              <th>Item</th>
              <th>Usage</th>
              <th>Cost</th>
            </tr>
          </thead>
          <tbody>
            <tr>
              <td>Push operations</td>
              <td>10 × 6 × 30 = 1,800 ops</td>
              <td>$0.02</td>
            </tr>
            <tr>
              <td>Storage</td>
              <td>15 GiB × 1 month</td>
              <td>$0.30</td>
            </tr>
            <tr>
              <td>Pull operations (CDN)</td>
              <td>300 × 6 × 30 = 54,000 ops</td>
              <td>$0.11</td>
            </tr>
            <tr className={styles.exampleTableSubtotal}>
              <td>Subtotal</td>
              <td></td>
              <td>$0.43</td>
            </tr>
            <tr>
              <td>Free tier credit</td>
              <td></td>
              <td>−{pricingDisplay.freeCredit}</td>
            </tr>
            <tr className={styles.exampleTableTotal}>
              <td>Total</td>
              <td></td>
              <td>$0.00</td>
            </tr>
          </tbody>
        </table>
      </section>

      <footer className={styles.footer}>
        <p>
          bin<sub>2</sub> &copy; 2025 &middot; <a href="#">terms</a> &middot;{" "}
          <a href="#">privacy</a>
        </p>
      </footer>
    </div>
  );
}
