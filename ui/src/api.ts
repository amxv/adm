const BASE = "/api/v1";

export interface Agent {
  name: string;
  task: string;
  status: "online" | "stale" | "unknown";
  last_seen_at: string;
}

export interface DeliverySummary {
  pending: number;
  offered: number;
  delivered: number;
  total: number;
}

export interface Message {
  id: string;
  from: string;
  kind: "direct" | "broadcast";
  body: string;
  created_at: string;
  recipients: string[];
  delivery: DeliverySummary;
}

export interface Receipt {
  recipient: string;
  state: "pending" | "offered" | "delivered";
  batch_token?: string;
  offered_at?: string;
  delivered_at?: string;
}

export interface MessageDetail {
  id: string;
  from: string;
  kind: "direct" | "broadcast";
  body: string;
  created_at: string;
  receipts: Receipt[];
}

export interface Claim {
  agent_name: string;
  path_pattern: string;
  path_norm: string;
  created_at: string;
  updated_at: string;
}

export interface PageInfo {
  limit: number;
  offset: number;
  total: number;
}

export interface MessagesResponse {
  items: Message[];
  page: PageInfo;
}

export interface MessageFilters {
  q?: string;
  from?: string;
  to?: string;
  kind?: string;
  state?: string;
  from_ts?: string;
  to_ts?: string;
  limit?: number;
  offset?: number;
}

async function get<T>(path: string, params?: Record<string, string>): Promise<T> {
  const url = new URL(BASE + path, window.location.origin);
  if (params) {
    for (const [k, v] of Object.entries(params)) {
      if (v) url.searchParams.set(k, v);
    }
  }
  const res = await fetch(url.toString());
  if (!res.ok) throw new Error(`${res.status} ${res.statusText}`);
  return res.json();
}

export interface ConflictPair {
  claim_a: Claim;
  claim_b: Claim;
  overlap_type: "exact" | "subset" | "mutual" | "glob";
}

export interface ConflictsResponse {
  conflicts: ConflictPair[];
  total: number;
}

export interface SyncBatch {
  token: string;
  agent_name: string;
  created_at: string;
}

export interface DeliveryDebugResponse {
  receipts: DeliverySummary;
  recent_batches: SyncBatch[];
}

export const api = {
  agents: () => get<Agent[]>("/agents"),
  claims: () => get<Claim[]>("/claims"),
  claimConflicts: () => get<ConflictsResponse>("/claims/conflicts"),
  deliveryDebug: () => get<DeliveryDebugResponse>("/debug/delivery"),
  messages: (filters?: MessageFilters) => {
    const params: Record<string, string> = {};
    if (filters) {
      for (const [k, v] of Object.entries(filters)) {
        if (v !== undefined && v !== "") params[k] = String(v);
      }
    }
    return get<MessagesResponse>("/messages", params);
  },
  message: (id: string) => get<MessageDetail>(`/messages/${id}`),
};
