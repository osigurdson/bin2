'use client';

import { useCreateRegistry, useGetRegistryExists } from "@/api/registries/hooks";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";

export default function NewRegistry() {
  const router = useRouter();
  const [name, setName] = useState('');
  const debouncedName = useDebounce(name, 400);
  const { data: exists } = useGetRegistryExists(debouncedName);
  const { mutate, isPending } = useCreateRegistry((created) => {
    router.push(`/dashboard/${created.id}`)
  });

  const canSave = !exists && !!name && !isPending;

  const onNameChange = (n: string) => {
    const slug = n.toLowerCase().replace(/[^a-z0-9-_]/g, "")
    if (slug === name) {
      return;
    }
    setName(slug);
  }

  const onSave = () => {
    mutate({ name: name });
  }

  return (
    <div className="flex flex-col items-center justify-center">
      <div className="flex flex-col w-md">
        <p>Welcome to bin<sub>2</sub>! We think you will like it very much. It
          uses an ultra-fast, global CDN backed system for pulling images, with
          low cost storage and no associated egress charges.
        </p>
        <p className="mt-4">
          Enter a new registry name below to get started. We'll ensure that
          the name is unique and will create everything needed for you to
          get started quickly.
        </p>

        <div className="flex items-center mt-4">
          <span>bin2.io/</span>
          <input
            className="bg-transparent border-1 outline-none p-0 m-0 w-auto font-semibold"
            spellCheck={false}
            autoCorrect="off"
            autoCapitalize="none"
            type="text"
            maxLength={64}
            value={name}
            onChange={(e) => onNameChange(e.target.value)}
          />
          {exists && (
            <span className="ml-2 text-error font-semibold ">That name is unavailable. Try a different one.</span>
          )}
        </div>
        <button
          className="btn btn-primary mt-4 w-10px"
          disabled={!canSave}
          onClick={onSave}
        >
          {isPending ? 'Creating...' : 'Create Registry'}
        </button>
      </div>
    </div>
  );
}

function useDebounce<T>(value: T, delay: number) {
  const [debounced, setDebounced] = useState(value);

  useEffect(() => {
    const timer = setTimeout(() => setDebounced(value), delay);
    return () => clearTimeout(timer);
  }, [value, delay]);

  return debounced;
}
