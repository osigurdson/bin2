import RegistryView from "./RegistryView";

export default async function Page({
  params,
}: {
  params: Promise<{ registryId: string }>;
}) {
  const { registryId } = await params;
  return (
    <div>
      <RegistryView registryId={registryId} />
    </div>
  )
}
