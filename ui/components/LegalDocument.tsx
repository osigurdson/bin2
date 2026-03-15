type LegalDocumentProps = {
  title: string;
  summary: React.ReactNode;
  lastUpdated: string;
  children: React.ReactNode;
};

type LegalSectionProps = {
  title: string;
  children: React.ReactNode;
};

export default function LegalDocument({
  title,
  summary,
  lastUpdated,
  children,
}: LegalDocumentProps) {
  return (
    <article className="py-10">
      <div className="flex flex-col gap-4 border-b border-base-200 pb-8">
        <p className="text-xs uppercase tracking-[2px] text-base-content/40">Legal</p>
        <h1 className="text-4xl font-bold">{title}</h1>
        <p className="max-w-2xl text-base-content/60 leading-7">{summary}</p>
        <p className="text-sm text-base-content/40">Last updated {lastUpdated}</p>
      </div>

      <div className="pt-8 space-y-10 [&_a]:underline [&_a]:underline-offset-2 [&_li]:pl-1 [&_ol]:list-decimal [&_ol]:space-y-2 [&_ol]:pl-5 [&_p]:leading-7 [&_p]:text-base-content/60 [&_ul]:list-disc [&_ul]:space-y-2 [&_ul]:pl-5">
        {children}
      </div>
    </article>
  );
}

export function LegalSection({ title, children }: LegalSectionProps) {
  return (
    <section className="space-y-3">
      <h2 className="text-xl font-bold">{title}</h2>
      {children}
    </section>
  );
}
