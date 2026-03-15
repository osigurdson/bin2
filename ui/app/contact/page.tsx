import { getSignInUrl, withAuth } from "@workos-inc/authkit-nextjs";
import MarketingLayout from "@/components/MarketingLayout";
import ContactForm from "./ContactForm";

export const metadata = {
  title: "contact - bin2",
  description: "Contact the bin2 team.",
};

export default async function ContactPage() {
  const { user } = await withAuth();
  const signInUrl = await getSignInUrl();
  const authenticatedIdentity = user?.email
    ? {
        name: user.firstName?.trim() || user.email,
        email: user.email,
      }
    : null;

  return (
    <MarketingLayout user={user} signInUrl={signInUrl}>
      <div className="py-10">
        <ContactForm authenticatedIdentity={authenticatedIdentity} />
      </div>
    </MarketingLayout>
  );
}
