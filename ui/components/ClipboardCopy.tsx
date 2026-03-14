import { useEffect, useRef, useState } from "react";
import { Check, Copy } from "lucide-react";

export default function ClipboardCopy({ copyText, className = "btn btn-sm btn-ghost" }: { copyText: string; className?: string }) {
  const [copied, setCopied] = useState(false);
  const resetTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    return () => {
      if (resetTimerRef.current) {
        clearTimeout(resetTimerRef.current);
      }
    };
  }, []);

  const handleCopy = async () => {
    await navigator.clipboard.writeText(copyText);
    setCopied(true);
    if (resetTimerRef.current) {
      clearTimeout(resetTimerRef.current);
    }
    resetTimerRef.current = setTimeout(() => {
      setCopied(false);
    }, 2000);
  };

  return (
    <button
      className={className}
      type="button"
      title="Copy key"

      onClick={handleCopy}>
      {copied ? <Check className="text-success" size={14} /> : <Copy size={14} />}
    </button>
  );
}
