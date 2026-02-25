import Registry from "./Registry";

export default async function Page({
  params,
}: {
  params: Promise<{ registryId: string }>;
}) {
  const { registryId } = await params;
  return (
    <div>
      <Registry registryId={registryId} />
    </div>
  )
}
