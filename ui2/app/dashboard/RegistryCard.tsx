import Link from "next/link";
import { formatBytes } from "@/lib/formatBytes";

type RegistryCardProps = {
  id: string;
  name: string;
  sizeBytes: number;
};

export default function RegistryCard(props: RegistryCardProps) {
  return (
    <Link
      href={`/dashboard/${props.id}`}
      className="flex items-center justify-between rounded-md px-3 py-2 hover:bg-base-200 transition-colors"
    >
      <span className="font-medium">bin2.io/<b>{props.name}</b></span>
      <span >{formatBytes(props.sizeBytes)}</span>
    </Link>
  );
}
