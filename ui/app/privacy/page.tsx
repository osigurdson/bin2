import { getSignInUrl, withAuth } from "@workos-inc/authkit-nextjs";
import MarketingLayout from "@/components/MarketingLayout";
import LegalDocument, { LegalSection } from "@/components/LegalDocument";

const lastUpdated = "March 14, 2026";

export const metadata = {
  title: "privacy - bin2",
  description: "Privacy policy for the hosted bin2 container registry.",
};

export default async function PrivacyPage() {
  const { user } = await withAuth();
  const signInUrl = await getSignInUrl();

  return (
    <MarketingLayout user={user} signInUrl={signInUrl}>
      <LegalDocument
        title="Privacy Policy"
        lastUpdated={lastUpdated}
        summary={
          <>
            This policy explains how bin2 collects, uses, and shares information when you
            use the hosted container registry service, website, documentation, and related
            account features.
          </>
        }
      >
        <LegalSection title="1. Information bin2 collects">
          <p>bin2 collects information needed to run the service, including:</p>
          <ul>
            <li>account and sign-in data, such as basic profile details and organization identifiers provided through the authentication provider;</li>
            <li>service data, such as registry names, repository names, tags, digests, manifests, blobs, artifact metadata, and deletion events;</li>
            <li>API key records, including key names, prefixes, scopes, creation timestamps, and last-used timestamps;</li>
            <li>usage and billing data, such as push operations, pull operations, stored-byte metrics, and related timestamps;</li>
            <li>technical and security data, such as IP addresses, user agents, request metadata, and audit or error logs generated while operating the service.</li>
          </ul>
        </LegalSection>

        <LegalSection title="2. How bin2 uses information">
          <p>bin2 uses collected information to:</p>
          <ul>
            <li>authenticate users and provision account access;</li>
            <li>store, replicate, and deliver container images and other OCI artifacts;</li>
            <li>measure usage, generate billing data, and enforce product limits;</li>
            <li>detect abuse, investigate incidents, and maintain service reliability;</li>
            <li>communicate operational notices, security updates, and product changes.</li>
          </ul>
        </LegalSection>

        <LegalSection title="3. How information is shared">
          <p>
            bin2 may share information with service providers that help operate the
            service, including identity, hosting, storage, and content-delivery
            infrastructure providers. For example, bin2 uses WorkOS for authentication and
            identity-related account flows.
          </p>
          <p>
            bin2 may also disclose information when reasonably necessary to comply with
            law, respond to legal process, protect the service or other users, or support
            a reorganization, merger, financing, or asset transfer involving the service.
          </p>
          <p>
            bin2 is not built to sell personal information or share it for cross-context
            behavioral advertising.
          </p>
        </LegalSection>

        <LegalSection title="4. Data retention">
          <p>
            bin2 keeps account and service data for as long as needed to provide the
            service, maintain security, support billing records, and satisfy legal or
            operational obligations.
          </p>
          <p>
            Registry content, metadata, and API key records are generally retained until
            you delete them or the related account is removed. Backups, caches, and
            security logs may persist for a limited period afterward.
          </p>
        </LegalSection>

        <LegalSection title="5. Security">
          <p>
            bin2 uses reasonable administrative, technical, and organizational safeguards
            intended to protect data. This includes transport security for the hosted
            service and encrypted storage for API key secrets at rest.
          </p>
          <p>
            No system is perfectly secure, and bin2 cannot guarantee absolute security of
            stored or transmitted information.
          </p>
        </LegalSection>

        <LegalSection title="6. Your choices">
          <p>
            You can manage registries and API keys through the product interface, including
            deleting registries and rotating or revoking credentials when needed.
          </p>
          <p>
            If you need access, correction, export, or deletion assistance for account
            information, use the support or account-management channel made available by
            the operator of your bin2 deployment.
          </p>
        </LegalSection>

        <LegalSection title="7. International and third-party processing">
          <p>
            bin2 may process data in multiple regions through infrastructure and CDN
            providers. By using the service, you understand that your information may be
            processed outside your home jurisdiction.
          </p>
          <p>
            Third-party services integrated with bin2, including your identity provider,
            may have their own privacy practices. bin2 does not control those separate
            policies.
          </p>
        </LegalSection>

        <LegalSection title="8. Children">
          <p>
            bin2 is intended for business and technical users and is not directed to
            children under 13.
          </p>
        </LegalSection>

        <LegalSection title="9. Changes to this policy">
          <p>
            bin2 may update this Privacy Policy from time to time. The current version
            will be posted on this page with a revised update date when changes take
            effect.
          </p>
        </LegalSection>
      </LegalDocument>
    </MarketingLayout>
  );
}
