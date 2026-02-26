'use client';

import { useGetRegistries } from "@/api/registries/hooks";
import RegistryCard from "./RegistryCard";
export default function Dashboard() {
  const { data } = useGetRegistries();
  if (!data) {
    return null;
  }

  if (data.registries.length == 0) {
    return <div>no reg</div>;
  }

  return (
    <div>
      <ul>
        {data.registries.map((registry) => (
          <li><RegistryCard name={registry.name} /></li>
        ))}
      </ul>
    </div>
  );
}
