'use client';

import { useCreateRegistry, useGetRegistryExists } from "@/api/registries/hooks";
import { useGetCurrentUser } from "@/api/users/hooks";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";

export default function NewRegistry() {
  const router = useRouter();
  const [name, setName] = useState('');
  const [suppressAvailability, setSuppressAvailability] = useState(false);
  const debouncedName = useDebounce(name, 400);
  const { data: currentUser, isLoading: isLoadingCurrentUser } = useGetCurrentUser();
  const { mutate, isPending } = useCreateRegistry((created) => {
    router.push(`/dashboard/${created.id}`);
  });
  const registryNameForAvailabilityCheck = isPending || suppressAvailability ? '' : debouncedName;
  const { data: exists } = useGetRegistryExists(registryNameForAvailabilityCheck);

  const canSave = !exists && !!name && !isPending;

  const onNameChange = (n: string) => {
    const slug = n.toLowerCase().replace(/[^a-z0-9-_]/g, "")
    if (slug === name) {
      return;
    }
    setSuppressAvailability(false);
    setName(slug);
  }

  const onSave = () => {
    if (!canSave) {
      return;
    }
    setSuppressAvailability(true);
    mutate(
      { name: name },
      {
        onError: () => {
          setSuppressAvailability(false);
        },
      }
    );
  }

  return (
    <div className="flex flex-col items-center justify-center">
      <form
        className="flex flex-col w-md"
        onSubmit={(e) => {
          e.preventDefault();
          onSave();
        }}
      >
        {!isLoadingCurrentUser && !currentUser?.onboarded && (
          <>
            <p>Welcome to bin<sub>2</sub>. Create your first registry to finish setup.</p>
            <p className="mt-4">
              It uses an ultra-fast, global CDN backed system for pulling images,
              with low cost storage and no associated egress charges.
            </p>
          </>
        )}
        <p className={(!isLoadingCurrentUser && !currentUser?.onboarded) ? "mt-4" : ""}>
          Enter a new registry name below. We&apos;ll ensure that the name is unique and
          create everything needed for you to get started quickly.
        </p>

        <div className="flex items-center mt-4">
          <span>bin2.io/</span>
          <input
            className="bg-transparent border-1 outline-none p-0 m-0 w-auto font-semibold"
            spellCheck={false}
            autoCorrect="off"
            autoCapitalize="none"
            autoFocus
            type="text"
            maxLength={64}
            value={name}
            onChange={(e) => onNameChange(e.target.value)}
          />
          {!isPending && !suppressAvailability && exists && (
            <span className="ml-2 text-error font-semibold ">That name is unavailable. Try a different one.</span>
          )}
        </div>
        <button
          className="btn btn-primary mt-4 w-10px"
          type="submit"
          disabled={!canSave}
        >
          {isPending ? 'Creating...' : 'Create Registry'}
        </button>
      </form>
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
