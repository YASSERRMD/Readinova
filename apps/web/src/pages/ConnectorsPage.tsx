import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { connectorsApi, type ConnectorConfig } from "../api/connectors";

function extractApiError(err: unknown): string {
  return (
    (err as { response?: { data?: { error?: string } } })?.response?.data
      ?.error ??
    (err as { message?: string })?.message ??
    "An unexpected error occurred"
  );
}

const KNOWN_CONNECTORS = [
  {
    type: "test",
    label: "Test Connector",
    description: "Synthetic signals for dev and testing.",
  },
  {
    type: "azure",
    label: "Azure",
    description: "Read-only ARM and Graph signals.",
  },
];

export default function ConnectorsPage() {
  const qc = useQueryClient();
  const { data: configs = [], isLoading } = useQuery({
    queryKey: ["connectors"],
    queryFn: connectorsApi.list,
    retry: false,
  });

  const configByType = Object.fromEntries(
    configs.map((c: ConnectorConfig) => [c.connector_type, c]),
  );

  const syncMutation = useMutation({
    mutationFn: (type: string) => connectorsApi.sync(type),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["connectors"] });
    },
  });

  const toggleMutation = useMutation({
    mutationFn: ({ type, enabled }: { type: string; enabled: boolean }) =>
      connectorsApi.upsert(type, { enabled }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["connectors"] });
    },
  });

  const [syncing, setSyncing] = useState<string | null>(null);
  const [connectError, setConnectError] = useState<string | null>(null);
  const [syncError, setSyncError] = useState<string | null>(null);

  async function handleConnect(type: string, label: string) {
    setConnectError(null);
    try {
      await connectorsApi.upsert(type, {
        display_name: label,
        credentials: {},
      });
      qc.invalidateQueries({ queryKey: ["connectors"] });
    } catch (err) {
      setConnectError(extractApiError(err));
    }
  }

  async function handleSync(type: string) {
    setSyncing(type);
    setSyncError(null);
    try {
      await syncMutation.mutateAsync(type);
    } catch (err) {
      setSyncError(extractApiError(err));
    } finally {
      setSyncing(null);
    }
  }

  if (isLoading) {
    return <div className="p-8 text-surface-400">Loading connectors…</div>;
  }

  return (
    <div className="p-8 max-w-4xl mx-auto space-y-6">
      <div>
        <h1 className="text-2xl font-bold text-white">Evidence Connectors</h1>
        <p className="text-surface-400 mt-1">
          Connect external systems to automatically collect evidence signals for
          your assessments.
        </p>
      </div>

      {connectError && (
        <p className="rounded-md bg-red-900/40 px-3 py-2 text-sm text-red-400">
          {connectError}
        </p>
      )}
      {syncError && (
        <p className="rounded-md bg-red-900/40 px-3 py-2 text-sm text-red-400">
          {syncError}
        </p>
      )}

      <div className="space-y-4">
        {KNOWN_CONNECTORS.map(({ type, label, description }) => {
          const config = configByType[type];
          const isConfigured = Boolean(config);
          const isEnabled = config?.enabled ?? false;

          return (
            <div
              key={type}
              className="card flex items-start justify-between gap-4"
            >
              <div className="flex-1 min-w-0">
                <div className="flex items-center gap-2">
                  <span className="font-semibold text-white">{label}</span>
                  {isConfigured && (
                    <span
                      className={`text-xs px-2 py-0.5 rounded-full ${isEnabled ? "bg-green-900 text-green-300" : "bg-surface-700 text-surface-400"}`}
                    >
                      {isEnabled ? "Enabled" : "Disabled"}
                    </span>
                  )}
                </div>
                <p className="text-surface-400 text-sm mt-0.5">{description}</p>
                {config?.last_sync_at && (
                  <p className="text-surface-500 text-xs mt-1">
                    Last sync: {new Date(config.last_sync_at).toLocaleString()}
                  </p>
                )}
                {config?.last_sync_error && (
                  <p className="text-red-400 text-xs mt-1">
                    Error: {config.last_sync_error}
                  </p>
                )}
              </div>

              <div className="flex items-center gap-2 shrink-0">
                {!isConfigured ? (
                  <button
                    className="btn-primary text-sm"
                    onClick={() => handleConnect(type, label)}
                  >
                    Connect
                  </button>
                ) : (
                  <>
                    <button
                      className="btn-ghost text-sm"
                      onClick={() =>
                        toggleMutation.mutate({ type, enabled: !isEnabled })
                      }
                    >
                      {isEnabled ? "Disable" : "Enable"}
                    </button>
                    <button
                      className="btn-primary text-sm"
                      disabled={!isEnabled || syncing === type}
                      onClick={() => handleSync(type)}
                    >
                      {syncing === type ? "Syncing…" : "Sync Now"}
                    </button>
                  </>
                )}
              </div>
            </div>
          );
        })}
      </div>

      <EvidencePanel />
    </div>
  );
}

function EvidencePanel() {
  const { data: signals = [] } = useQuery({
    queryKey: ["evidence"],
    queryFn: () => connectorsApi.listEvidence(),
    retry: false,
  });

  if (signals.length === 0) return null;

  return (
    <div className="space-y-3">
      <h2 className="text-lg font-semibold text-white">
        Recent Evidence Signals
      </h2>
      <div className="overflow-auto rounded-lg border border-surface-700">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-surface-700 text-surface-400 text-left">
              <th className="px-4 py-2">Connector</th>
              <th className="px-4 py-2">Dimension</th>
              <th className="px-4 py-2">Signal</th>
              <th className="px-4 py-2">Value</th>
              <th className="px-4 py-2">Collected</th>
            </tr>
          </thead>
          <tbody>
            {signals.map((s) => (
              <tr
                key={`${s.connector_type}:${s.signal_key}`}
                className="border-b border-surface-800 hover:bg-surface-800/50"
              >
                <td className="px-4 py-2 text-surface-300">
                  {s.connector_type}
                </td>
                <td className="px-4 py-2 text-surface-300">
                  {s.dimension_slug}
                </td>
                <td className="px-4 py-2 text-white font-mono text-xs">
                  {s.signal_key}
                </td>
                <td className="px-4 py-2 text-brand-400 font-mono text-xs">
                  {JSON.stringify(s.signal_value)}
                </td>
                <td className="px-4 py-2 text-surface-500 text-xs">
                  {new Date(s.collected_at).toLocaleString()}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
