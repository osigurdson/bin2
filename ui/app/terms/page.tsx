import { getSignInUrl, withAuth } from "@workos-inc/authkit-nextjs";
import MarketingLayout from "@/components/MarketingLayout";
import LegalDocument, { LegalSection } from "@/components/LegalDocument";

const lastUpdated = "March 14, 2026";

export const metadata = {
  title: "terms - bin2",
  description: "Terms of service for the hosted bin2 container registry.",
};

export default async function TermsPage() {
  const { user } = await withAuth();
  const signInUrl = await getSignInUrl();

  return (
    <MarketingLayout user={user} signInUrl={signInUrl}>
      <LegalDocument
        title="Terms of Service"
        lastUpdated={lastUpdated}
        summary={
          <>
            These Terms of Service govern access to the hosted bin2 service. bin2 is a
            managed OCI-compatible container registry with separate push and pull
            endpoints, usage-based billing, and account-scoped API keys.
          </>
        }
      >
        <LegalSection title="1. Using the service">
          <p>
            By using bin2, you agree to these terms. If you use the service on behalf of
            a company or team, you represent that you have authority to bind that
            organization.
          </p>
          <p>
            You must use the service in compliance with applicable law and only for
            legitimate software distribution, build, deployment, and artifact management
            workflows.
          </p>
        </LegalSection>

        <LegalSection title="2. Accounts and credentials">
          <p>
            You are responsible for keeping account credentials, sessions, and API keys
            secure. Activity performed through your account or keys is your
            responsibility unless caused by bin2&apos;s own breach of these terms.
          </p>
          <p>
            You must promptly revoke or rotate credentials that you believe were exposed,
            and you must provide accurate account information needed to operate your
            workspace.
          </p>
        </LegalSection>

        <LegalSection title="3. Your content">
          <p>
            You retain ownership of the images, artifacts, manifests, layers, tags, and
            related metadata you upload to bin2. You grant bin2 a limited right to host,
            copy, cache, replicate, transmit, and process that content solely as needed
            to operate, secure, and improve the service, including delivery through CDN
            infrastructure.
          </p>
          <p>
            You are responsible for the legality, integrity, and accuracy of content you
            publish through the service, including making sure you have the rights needed
            to store and distribute it.
          </p>
        </LegalSection>

        <LegalSection title="4. Acceptable use">
          <p>You may not use bin2 to:</p>
          <ul>
            <li>violate law or infringe another party&apos;s intellectual property rights;</li>
            <li>store or distribute malware, ransomware, or other harmful payloads;</li>
            <li>probe, disrupt, overload, or bypass the security or capacity limits of the service;</li>
            <li>misuse shared infrastructure in a way that degrades service for other users;</li>
            <li>attempt unauthorized access to registries, repositories, or data that are not yours.</li>
          </ul>
        </LegalSection>

        <LegalSection title="5. Metering, pricing, and payment">
          <p>
            bin2 measures usage using the pricing model published on the marketing site
            and documentation, including push operations, pull operations, and stored
            bytes over time. If your account incurs paid usage, you are responsible for
            those charges.
          </p>
          <p>
            Free usage tiers, pricing, and product features may change prospectively. If
            bin2 materially changes pricing for future usage, the updated pricing will be
            posted before it applies.
          </p>
        </LegalSection>

        <LegalSection title="6. Availability and changes">
          <p>
            bin2 may change, suspend, or limit parts of the service to maintain security,
            capacity, compliance, or product quality. Unless separately agreed in writing,
            the service is offered without a guaranteed service level.
          </p>
          <p>
            We may perform maintenance, rotate infrastructure, or remove features that are
            no longer viable. We will generally try to avoid unnecessary disruption, but
            some changes may take effect immediately when required for safety or legal
            reasons.
          </p>
        </LegalSection>

        <LegalSection title="7. Suspension, termination, and deletion">
          <p>
            You may stop using bin2 at any time. bin2 may suspend or terminate access if
            you violate these terms, create material risk for the service or other users,
            or fail to pay applicable charges.
          </p>
          <p>
            Deleting a registry or repository may permanently remove associated data. bin2
            is not required to recover content after deletion, termination, or expiration
            of applicable retention periods.
          </p>
        </LegalSection>

        <LegalSection title="8. Disclaimers">
          <p>
            To the maximum extent permitted by law, bin2 is provided on an &quot;as is&quot;
            and &quot;as available&quot; basis. bin2 disclaims implied warranties of
            merchantability, fitness for a particular purpose, non-infringement, and any
            warranty that the service will be uninterrupted, error-free, or lossless.
          </p>
        </LegalSection>

        <LegalSection title="9. Limitation of liability">
          <p>
            To the maximum extent permitted by law, bin2 will not be liable for indirect,
            incidental, special, consequential, exemplary, or punitive damages, or for
            lost profits, revenues, goodwill, or data.
          </p>
          <p>
            bin2&apos;s total liability for any claim arising out of or relating to the
            service will not exceed the greater of the amount you paid to bin2 during the
            12 months before the event giving rise to the claim or $100 USD.
          </p>
        </LegalSection>

        <LegalSection title="10. Updates to these terms">
          <p>
            bin2 may update these terms from time to time. Updated terms become effective
            when posted on this page unless a later effective date is stated. Continued
            use of the service after an update means you accept the revised terms.
          </p>
        </LegalSection>
      </LegalDocument>
    </MarketingLayout>
  );
}
