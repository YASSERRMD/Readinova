import { apiClient } from './client';

export interface ConnectorConfig {
  connector_type: string;
  display_name: string;
  enabled: boolean;
  last_sync_at?: string;
  last_sync_error?: string;
  created_at: string;
}

export interface EvidenceSignal {
  connector_type: string;
  dimension_slug: string;
  signal_key: string;
  signal_value: unknown;
  collected_at: string;
}

export interface UpsertConnectorPayload {
  display_name?: string;
  credentials?: Record<string, unknown>;
  enabled?: boolean;
}

export const connectorsApi = {
  list(): Promise<ConnectorConfig[]> {
    return apiClient.get('/v1/connectors').then(r => r.data);
  },

  upsert(type: string, payload: UpsertConnectorPayload): Promise<{ connector_type: string; status: string }> {
    return apiClient.put(`/v1/connectors/${type}`, payload).then(r => r.data);
  },

  remove(type: string): Promise<void> {
    return apiClient.delete(`/v1/connectors/${type}`).then(() => undefined);
  },

  sync(type: string): Promise<{ connector_type: string; signals_synced: number }> {
    return apiClient.post(`/v1/connectors/${type}/sync`).then(r => r.data);
  },

  listEvidence(dimension?: string): Promise<EvidenceSignal[]> {
    const params = dimension ? { dimension } : {};
    return apiClient.get('/v1/evidence', { params }).then(r => r.data);
  },
};
