import Link from "next/link";
import { Trash2 } from "lucide-react";
import { formatBytes } from "@/lib/formatBytes";

type RegistryCardProps = {
  id: string;
  name: string;
  sizeBytes: number;
  onDelete?: (id: string, name: string) => void;
  isDeleting?: boolean;
};

export default function RegistryCard(props: RegistryCardProps) {
  return (
    <div className="flex items-center gap-3 rounded-md px-3 py-2 hover:bg-base-200 transition-colors">
      <Link
        href={`/dashboard/${props.id}`}
        className="min-w-0 flex-1 flex items-center justify-between gap-4"
      >
        <span className="font-medium">bin2.io/<b>{props.name}</b></span>
        <span>{formatBytes(props.sizeBytes)}</span>
      </Link>
      {props.onDelete && (
        <button
          type="button"
          className="btn btn-ghost btn-xs btn-square text-error"
          title={`Delete ${props.name}`}
          aria-label={`Delete ${props.name}`}
          disabled={props.isDeleting}
          onClick={() => props.onDelete?.(props.id, props.name)}
        >
          {props.isDeleting ? (
            <span className="loading loading-spinner loading-xs" aria-hidden />
          ) : (
            <Trash2 size={14} aria-hidden />
          )}
        </button>
      )}
    </div>
  );
}
