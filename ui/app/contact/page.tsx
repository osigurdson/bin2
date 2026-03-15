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

  return (
    <MarketingLayout user={user} signInUrl={signInUrl}>
      <div className="py-10">
        <ContactForm />
      </div>
    </MarketingLayout>
  );
}
