'use client';

import ClipboardCopy from "@/components/ClipboardCopy";
import { useState } from "react";

type CommandProps = {
  id: string;
  name: string;
  apiKey: string;
}

export default function Commands(props: CommandProps) {
  const [client, setClient] = useState<ClientType>('docker');
  const username = `bin2.io/${props.name}`;
  const cliLoginCommand = `${client} login -u ${username} -p ${props.apiKey}`;
  const pullSecretName = `${props.name}-pull-secret`;
  const pullSecretYaml = buildPullSecretYaml({
    name: pullSecretName,
    username,
    password: props.apiKey,
  });
  const maskedPullSecretYaml = buildMaskedPullSecretYaml({
    name: pullSecretName,
  });

  return (
    <div className="flex flex-col bg-base-200 p-2 gap-1">
      <span><b>Login</b></span>
      <div className={`flex ${client === 'k8s' ? 'items-start' : 'items-center'}`}>
        <ClientSelect value={client} onChange={setClient} />
        {client === 'k8s' ? (
          <>
            <pre className="text-xs">
              {maskedPullSecretYaml}
            </pre>
            <ClipboardCopy copyText={pullSecretYaml} />
          </>
        ) : (
          <>
            <span>login -u bin2.io/{props.name} -p ••••</span>
            <ClipboardCopy copyText={cliLoginCommand} />
          </>
        )}
      </div>
    </div>
  );
}

type ClientType = 'docker' | 'podman' | 'oras' | 'k8s';

type ClientSelectProps = {
  value: ClientType;
  onChange: (value: ClientType) => void;
}

function ClientSelect({ value, onChange }: ClientSelectProps) {
  return (
    <div>
      <select
        value={value}
        onChange={(e) => onChange(e.target.value as ClientType)}
        className="
        bg-transparent 
        outline-none 
        appearance-none 
        cursor-pointer 
        font-medium pr-2"
      >
        <option value="docker">docker⌄</option>
        <option value="podman">podman⌄</option>
        <option value="oras">oras⌄</option>
        <option value="k8s">k8s⌄</option>
      </select>
    </div>
  );
}

function buildPullSecretYaml({
  name,
  username,
  password,
}: {
  name: string;
  username: string;
  password: string;
}) {
  const registry = "bin2.io";
  const auth = btoa(`${username}:${password}`);
  const dockerConfigJson = JSON.stringify({
    auths: {
      [registry]: {
        username,
        password,
        auth,
      },
    },
  });
  const dockerConfigJsonBase64 = btoa(dockerConfigJson);

  return [
    "apiVersion: v1",
    "kind: Secret",
    "metadata:",
    `  name: ${name}`,
    "type: kubernetes.io/dockerconfigjson",
    "data:",
    `  .dockerconfigjson: ${dockerConfigJsonBase64}`,
  ].join("\n");
}

function buildMaskedPullSecretYaml({
  name,
}: {
  name: string;
}) {
  return [
    "apiVersion: v1",
    "kind: Secret",
    "metadata:",
    `  name: ${name}`,
    "type: kubernetes.io/dockerconfigjson",
    "data:",
    "  .dockerconfigjson: ••••",
  ].join("\n");
}
