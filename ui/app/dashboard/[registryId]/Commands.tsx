'use client';

import ClipboardCopy from "@/components/ClipboardCopy";
import ConfirmModal from "@/components/ConfirmModal";
import { ChevronDown, Check, Copy, RefreshCw } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { APIKey } from "@/api/apikeys/types";
import { getRegistryInfo } from "@/lib/runenv";
import { useCreateAPIKey, useDeleteAPIKey } from "@/api/apikeys/hooks";

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
  const { addr, pullAddr, isInsecure } = getRegistryInfo();
  const client = props.selectedClient;
  const activeKey = props.apiKeys[0];

  let tlsVerifyStr = '';
  if (client === 'podman' && isInsecure) {
    tlsVerifyStr = '--tls-verify=false'
  }

  const regUsername = 'bin2';
  const password = activeKey?.secretKey ?? '';
  const cliLoginCommand = `${client} login ${tlsVerifyStr} ${addr} -u ${regUsername} -p ${password}`;
  const pullSecretName = `${props.name}-pull-secret`;
  const pullSecret = buildPullSecret({
    name: pullSecretName,
    username: regUsername,
    password,
    registry: pullAddr,
  });
  const pullSecretYaml = pullSecret.yaml;
  const maskedPullSecretYamlHeader = buildMaskedPullSecretYamlHeader({
    name: pullSecretName,
  });

  return (
    <div className="flex flex-col bg-base-200 p-2 gap-1">
      <span><b>Login</b></span>
      <div className={`flex ${client === 'k8s' ? 'items-start' : 'items-center'}`}>
        <ClientSelect value={client} onChange={props.selectedClientChangeAction} />
        {client === 'k8s' ? (
          <>
            <div className="text-xs font-mono flex-1 whitespace-pre">
              {maskedPullSecretYamlHeader}{"\n  .dockerconfigjson: "}
              <PasswordArea
                activeKey={activeKey}
                registryId={props.id}
                registryName={props.name}
                copyText={pullSecret.dockerConfigJsonBase64}
                copyTitle="Copy .dockerconfigjson"
              />
            </div>
            {activeKey && <ClipboardCopy copyText={pullSecretYaml} />}
          </>
        ) : (
          <>
            <span>login {tlsVerifyStr} {addr} -u {regUsername} -p&nbsp;</span>
            <PasswordArea activeKey={activeKey} registryId={props.id} registryName={props.name} />
            {activeKey && <ClipboardCopy copyText={cliLoginCommand} />}
          </>
        )}
      </div>
    </div>
  );
}

type PasswordAreaProps = {
  activeKey: APIKey | undefined;
  registryId: string;
  registryName: string;
  copyText?: string;
  copyTitle?: string;
};

// AKIA — API Key Interaction Area
function PasswordArea({ activeKey, registryId, registryName, copyText, copyTitle = 'Copy key' }: PasswordAreaProps) {
  const [open, setOpen] = useState(false);
  const [showRotateConfirm, setShowRotateConfirm] = useState(false);
  const [copied, setCopied] = useState(false);
  const closeTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const copyTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const rootRef = useRef<HTMLElement | null>(null);

  const createMutation = useCreateAPIKey();
  const deleteMutation = useDeleteAPIKey();

  const isPending = createMutation.isPending || deleteMutation.isPending;

  function scheduleClose() {
    closeTimer.current = setTimeout(() => setOpen(false), 120);
  }

  function cancelClose() {
    if (closeTimer.current) clearTimeout(closeTimer.current);
  }

  useEffect(() => {
    return () => {
      if (closeTimer.current) clearTimeout(closeTimer.current);
      if (copyTimer.current) clearTimeout(copyTimer.current);
    };
  }, []);

  useEffect(() => {
    if (!open) return;
    function handlePointerDown(e: PointerEvent) {
      if (!rootRef.current?.contains(e.target as Node)) setOpen(false);
    }
    document.addEventListener('pointerdown', handlePointerDown);
    return () => document.removeEventListener('pointerdown', handlePointerDown);
  }, [open]);

  async function handleCopy() {
    if (!activeKey) return;
    await navigator.clipboard.writeText(copyText ?? activeKey.secretKey);
    setCopied(true);
    if (copyTimer.current) clearTimeout(copyTimer.current);
    copyTimer.current = setTimeout(() => setCopied(false), 2000);
  }

  async function handleCreate() {
    const keyName = registryName.replace(/[^A-Za-z0-9._-]/g, '-').slice(0, 32);
    await createMutation.mutateAsync({
      keyName,
      scopes: [{ registryId, permission: 'write' }],
    });
    setOpen(false);
  }

  async function handleRotate() {
    if (!activeKey) return;
    await deleteMutation.mutateAsync(activeKey.id);
    await createMutation.mutateAsync({
      keyName: activeKey.keyName,
      scopes: [{ registryId, permission: activeKey.scopes.find(s => s.registryId === registryId)?.permission ?? 'write' }],
    });
    setShowRotateConfirm(false);
    setOpen(false);
  }

  return (
    <>
      <span
        ref={rootRef}
        className="relative inline-block"
        onMouseEnter={() => { cancelClose(); setOpen(true); }}
        onMouseLeave={scheduleClose}
      >
        <button
          type="button"
          onClick={() => setOpen(v => !v)}
          aria-label={activeKey ? 'Manage API key' : 'Create API key'}
          className={[
            'font-mono text-xs px-1.5 py-0.5 rounded transition-colors cursor-pointer',
            activeKey
              ? 'border border-primary/30 bg-primary/8 text-base-content/90 hover:border-primary/60 hover:bg-primary/15'
              : 'border border-dashed border-base-content/25 text-base-content/40 hover:border-base-content/50 hover:text-base-content/60',
          ].join(' ')}
        >
          {activeKey ? `sk_${activeKey.prefix.slice(0, 3)}…` : '—'}
        </button>

        {open && (
          <div
            className="absolute right-0 top-full z-20 mt-1 rounded-md border border-base-300 bg-base-100 p-1 shadow-sm flex flex-col"
            onMouseEnter={cancelClose}
            onMouseLeave={scheduleClose}
          >
            {activeKey ? (
              <>
                <button
                  type="button"
                  title={copyTitle}
                  onClick={handleCopy}
                  className="flex items-center rounded px-2 py-1.5 text-base-content/80 hover:bg-base-200"
                >
                  {copied ? <Check size={13} className="text-success" /> : <Copy size={13} />}
                </button>
                <button
                  type="button"
                  title="Rotate key"
                  onClick={() => setShowRotateConfirm(true)}
                  disabled={isPending}
                  className="flex items-center rounded px-2 py-1.5 text-base-content/80 hover:bg-base-200 disabled:opacity-40"
                >
                  <RefreshCw size={13} />
                </button>
              </>
            ) : (
              <button
                type="button"
                onClick={handleCreate}
                disabled={isPending}
                className="flex items-center rounded px-2 py-1.5 text-[13px] text-base-content/80 hover:bg-base-200 disabled:opacity-40"
              >
                {createMutation.isPending ? 'Creating…' : 'Create key'}
              </button>
            )}
          </div>
        )}
      </span>

      <ConfirmModal
        isOpen={showRotateConfirm}
        title="Rotate API Key"
        message="This will invalidate the current key immediately. Any service using it will lose access until updated."
        confirmLabel="Rotate"
        isConfirming={isPending}
        confirmAction={handleRotate}
        cancelAction={() => setShowRotateConfirm(false)}
      />
    </>
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

function buildPullSecret({
  name,
  username,
  password,
  registry,
}: {
  name: string;
  username: string;
  password: string;
  registry: string;
}) {
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

  return {
    dockerConfigJsonBase64,
    yaml: [
      "apiVersion: v1",
      "kind: Secret",
      "metadata:",
      `  name: ${name}`,
      "type: kubernetes.io/dockerconfigjson",
      "data:",
      `  .dockerconfigjson: ${dockerConfigJsonBase64}`,
    ].join("\n"),
  };
}

function buildMaskedPullSecretYamlHeader({
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
  ].join("\n");
}
