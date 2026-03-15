'use client';

import ClipboardCopy from "@/components/ClipboardCopy";
import { ChevronDown, Check } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { APIKey } from "@/api/apikeys/types";
import { getRegistryInfo } from "@/lib/runenv";

export type ClientType = 'docker' | 'podman' | 'oras' | 'k8s';

const clientOptions: { value: ClientType; label: string }[] = [
  { value: 'docker', label: 'docker' },
  { value: 'podman', label: 'podman' },
  { value: 'oras', label: 'oras' },
  { value: 'k8s', label: 'k8s' },
];

type CommandProps = {
  id: string;
  name: string;
  apiKeys: APIKey[];
  selectedClient: ClientType;
  selectedClientChangeAction: (value: ClientType) => void;
}

export default function Commands(props: CommandProps) {
  const { addr, isInsecure } = getRegistryInfo();
  const [selectedKeyId, setSelectedKeyId] = useState<string | null>(null);
  const client = props.selectedClient;
  const activeKey = selectedKeyId
    ? props.apiKeys.find(k => k.id === selectedKeyId) ?? props.apiKeys[0]
    : props.apiKeys[0];

  let tlsVerifyStr = '';
  if (client === 'podman' && isInsecure) {
    tlsVerifyStr = '--tls-verify=false'
  }

  const regUsername = 'bin2';
  const password = activeKey?.secretKey ?? '';
  const cliLoginCommand = `${client} login ${tlsVerifyStr} ${addr} -u ${regUsername} -p ${password}`;
  const pullSecretName = `${props.name}-pull-secret`;
  const pullSecretYaml = buildPullSecretYaml({
    name: pullSecretName,
    username: regUsername,
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
          className="bg-transparent outline-none appearance-none cursor-pointer text-sm font-[inherit]"
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
        <ClientSelect value={client} onChange={props.selectedClientChangeAction} />
        {client === 'k8s' ? (
          <>
            <pre className="text-xs">
              {maskedPullSecretYaml}
            </pre>
            <ClipboardCopy copyText={pullSecretYaml} />
          </>
        ) : (
          <>
            <span>login {tlsVerifyStr} {addr} -u {regUsername} -p {activeKey ? '••••' : '—'}</span>
            {activeKey && <ClipboardCopy copyText={cliLoginCommand} />}
          </>
        )}
      </div>
    </div>
  );
}

type ClientSelectProps = {
  value: ClientType;
  onChange: (value: ClientType) => void;
}

function ClientSelect({ value, onChange }: ClientSelectProps) {
  const [isOpen, setIsOpen] = useState(false);
  const rootRef = useRef<HTMLDivElement | null>(null);
  const selectedOption = clientOptions.find((option) => option.value === value) ?? clientOptions[0];

  useEffect(() => {
    function handlePointerDown(event: PointerEvent) {
      if (!rootRef.current?.contains(event.target as Node)) {
        setIsOpen(false);
      }
    }

    function handleKeyDown(event: KeyboardEvent) {
      if (event.key === 'Escape') {
        setIsOpen(false);
      }
    }

    document.addEventListener('pointerdown', handlePointerDown);
    document.addEventListener('keydown', handleKeyDown);

    return () => {
      document.removeEventListener('pointerdown', handlePointerDown);
      document.removeEventListener('keydown', handleKeyDown);
    };
  }, []);

  return (
    <div ref={rootRef} className="relative mr-2">
      <button
        type="button"
        aria-haspopup="menu"
        aria-expanded={isOpen}
        className="inline-flex items-center gap-1 text-[13px] font-medium leading-none text-base-content/80 hover:text-base-content focus-visible:outline-none"
        onClick={() => setIsOpen((current) => !current)}
      >
        <span>{selectedOption.label}</span>
        <ChevronDown size={13} aria-hidden className={`transition-transform ${isOpen ? 'rotate-180' : ''}`} />
      </button>
      {isOpen && (
        <div className="absolute left-0 top-full z-20 mt-2 min-w-24 rounded-md border border-base-300 bg-base-100 p-1 shadow-sm">
          {clientOptions.map((option) => (
            <button
              key={option.value}
              type="button"
              className="flex w-full items-center justify-between gap-3 rounded px-2 py-1.5 text-left text-[13px] text-base-content/80 hover:bg-base-200"
              onClick={() => {
                onChange(option.value);
                setIsOpen(false);
              }}
            >
              <span>{option.label}</span>
              <span className="w-3">
                {option.value === value ? <Check size={13} aria-hidden /> : null}
              </span>
            </button>
          ))}
        </div>
      )}
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
