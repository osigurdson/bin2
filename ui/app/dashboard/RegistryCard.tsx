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
  const isEmpty = !props.sizeBytes || props.sizeBytes === 0;

  return (
    <div className="flex items-center gap-3 border-b border-base-content/10 px-1 py-2.5">
      <Link
        href={`/dashboard/${props.id}`}
        className="min-w-0 flex-1 flex items-center gap-3"
      >
        <b>{props.name}</b>
      </Link>
      <div className="flex items-center gap-4">
        {isEmpty ? (
          <span className="text-sm italic opacity-40">Empty</span>
        ) : (
          <span className="text-sm opacity-60">{formatBytes(props.sizeBytes)}</span>
        )}
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
    </div>
  );
}
