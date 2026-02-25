export default async function Layout({
  children,
  params,
}: Readonly<{
  children: React.ReactNode;
  params: Promise<{ registryId: string }>;
}>) {
  const { registryId } = await params;

  return (
    <div className="flex flex-col h-dvh">
      {children}
    </div>
  )
}
