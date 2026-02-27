'use client';

import ClipboardCopy from "@/components/ClipboardCopy";
import { useState } from "react";
import { APIKey } from "@/api/apikeys/types";

type CommandProps = {
  id: string;
  name: string;
  apiKeys: APIKey[];
}

export default function Commands(props: CommandProps) {
  const [client, setClient] = useState<ClientType>('docker');
  const [selectedKeyId, setSelectedKeyId] = useState<string | null>(null);

  const activeKey = selectedKeyId
    ? props.apiKeys.find(k => k.id === selectedKeyId) ?? props.apiKeys[0]
    : props.apiKeys[0];

  const username = 'bin2';
  const password = activeKey?.secretKey ?? '';
  const cliLoginCommand = `${client} login push.bin2.io -u ${username} -p ${password}`;
  const pullSecretName = `${props.name}-pull-secret`;
  const pullSecretYaml = buildPullSecretYaml({
    name: pullSecretName,
    username,
    password,
  });
  const maskedPullSecretYaml = buildMaskedPullSecretYaml({
    name: pullSecretName,
  });

  return (
    <div className="flex flex-col bg-base-200 p-2 gap-1">
      <span><b>Login</b></span>
      {props.apiKeys.length > 1 && (
        <select
          value={activeKey?.id ?? ''}
          onChange={e => setSelectedKeyId(e.target.value)}
          className="bg-transparent outline-none appearance-none cursor-pointer text-sm"
        >
          {props.apiKeys.map(k => (
            <option key={k.id} value={k.id}>{k.keyName}</option>
          ))}
        </select>
      )}
      {props.apiKeys.length === 0 && (
        <span className="text-sm opacity-60">No API keys — create one to log in.</span>
      )}
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
            <span>login -u {username} -p {activeKey ? '••••' : '—'}</span>
            {activeKey && <ClipboardCopy copyText={cliLoginCommand} />}
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
