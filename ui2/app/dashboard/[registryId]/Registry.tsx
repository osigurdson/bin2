'use client';

import Header from "@/components/Header";

export default function Registry({ registryId }: { registryId: string }) {
  return (
    <>
      <Header />
      <div className="flex justify-center items-center ml-1 w-full">
        <div>
          {registryId}
        </div>
      </div>
    </>
  )
}
