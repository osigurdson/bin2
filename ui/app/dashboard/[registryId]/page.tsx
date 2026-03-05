import RegistryView from "./RegistryView";

export default async function Page({
  params,
}: {
  params: Promise<{ registryId: string }>;
}) {
  const { registryId } = await params;
  return <RegistryView registryId={registryId} />;
}
