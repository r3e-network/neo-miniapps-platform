import { supabaseServiceClient } from "./supabase.ts";
import { error } from "./response.ts";

export type EventsQueryParams = {
  app_id?: string;
  event_name?: string;
  limit?: number;
  after_id?: string;
};

export type TransactionsQueryParams = {
  app_id?: string;
  limit?: number;
  after_id?: string;
};

export type EventsListResponse = {
  events: Array<{
    id: string;
    tx_hash: string;
    block_index: number;
    contract_hash: string;
    event_name: string;
    app_id: string | null;
    state: Record<string, unknown> | null;
    created_at: string;
  }>;
  has_more: boolean;
  last_id: string | null;
};

export type TransactionsListResponse = {
  transactions: Array<{
    id: string;
    tx_hash: string | null;
    request_id: string;
    from_service: string;
    tx_type: string;
    contract_address: string;
    method_name: string;
    params: Record<string, unknown>;
    gas_consumed: number | null;
    status: string;
    retry_count: number;
    error_message: string | null;
    rpc_endpoint: string | null;
    submitted_at: string;
    confirmed_at: string | null;
  }>;
  has_more: boolean;
  last_id: string | null;
};

function parseLimit(raw: string | null, defaultLimit: number, maxLimit: number): number {
  if (!raw) return defaultLimit;
  const n = Number.parseInt(raw, 10);
  if (!Number.isFinite(n) || n <= 0) return defaultLimit;
  return Math.min(n, maxLimit);
}

export async function queryEvents(params: EventsQueryParams, req?: Request): Promise<EventsListResponse | Response> {
  const limit = parseLimit(String(params.limit ?? ""), 100, 1000);
  const afterId = params.after_id ? String(params.after_id).trim() : undefined;

  const supabase = supabaseServiceClient();
  let query = supabase
    .from("contract_events")
    .select("*")
    .order("id", { ascending: false })
    .limit(limit + 1);

  if (params.app_id) {
    const appId = String(params.app_id).trim();
    if (!appId) return error(400, "app_id cannot be empty", "INVALID_PARAM", req);
    query = query.eq("app_id", appId);
  }

  if (params.event_name) {
    const eventName = String(params.event_name).trim();
    if (!eventName) return error(400, "event_name cannot be empty", "INVALID_PARAM", req);
    query = query.eq("event_name", eventName);
  }

  if (afterId) {
    const afterIdNum = Number.parseInt(afterId, 10);
    if (!Number.isFinite(afterIdNum) || afterIdNum <= 0) {
      return error(400, "after_id must be a positive integer", "INVALID_PARAM", req);
    }
    query = query.lt("id", afterIdNum);
  }

  const { data, error: queryErr } = await query;
  if (queryErr) return error(500, `failed to query events: ${queryErr.message}`, "DB_ERROR", req);

  const events = (data ?? []).slice(0, limit);
  const hasMore = (data ?? []).length > limit;
  const lastId = events.length > 0 ? String((events[events.length - 1] as any)?.id ?? "") : null;

  return {
    events: events.map((row: any) => ({
      id: String(row.id ?? ""),
      tx_hash: String(row.tx_hash ?? ""),
      block_index: Number(row.block_index ?? 0),
      contract_hash: String(row.contract_hash ?? ""),
      event_name: String(row.event_name ?? ""),
      app_id: row.app_id ? String(row.app_id) : null,
      state: row.state ?? null,
      created_at: String(row.created_at ?? ""),
    })),
    has_more: hasMore,
    last_id: lastId,
  };
}

export async function queryTransactions(
  params: TransactionsQueryParams,
  req?: Request,
): Promise<TransactionsListResponse | Response> {
  const limit = parseLimit(String(params.limit ?? ""), 100, 1000);
  const afterId = params.after_id ? String(params.after_id).trim() : undefined;

  const supabase = supabaseServiceClient();
  let query = supabase
    .from("chain_txs")
    .select("*")
    .order("id", { ascending: false })
    .limit(limit + 1);

  if (params.app_id) {
    const appId = String(params.app_id).trim();
    if (!appId) return error(400, "app_id cannot be empty", "INVALID_PARAM", req);
    // Filter by request_id pattern (assumes request_id contains app_id)
    query = query.ilike("request_id", `%${appId}%`);
  }

  if (afterId) {
    const afterIdNum = Number.parseInt(afterId, 10);
    if (!Number.isFinite(afterIdNum) || afterIdNum <= 0) {
      return error(400, "after_id must be a positive integer", "INVALID_PARAM", req);
    }
    query = query.lt("id", afterIdNum);
  }

  const { data, error: queryErr } = await query;
  if (queryErr) return error(500, `failed to query transactions: ${queryErr.message}`, "DB_ERROR", req);

  const transactions = (data ?? []).slice(0, limit);
  const hasMore = (data ?? []).length > limit;
  const lastId = transactions.length > 0 ? String((transactions[transactions.length - 1] as any)?.id ?? "") : null;

  return {
    transactions: transactions.map((row: any) => ({
      id: String(row.id ?? ""),
      tx_hash: row.tx_hash ? String(row.tx_hash) : null,
      request_id: String(row.request_id ?? ""),
      from_service: String(row.from_service ?? ""),
      tx_type: String(row.tx_type ?? ""),
      contract_address: String(row.contract_address ?? ""),
      method_name: String(row.method_name ?? ""),
      params: row.params ?? {},
      gas_consumed: row.gas_consumed !== null && row.gas_consumed !== undefined ? Number(row.gas_consumed) : null,
      status: String(row.status ?? ""),
      retry_count: Number(row.retry_count ?? 0),
      error_message: row.error_message ? String(row.error_message) : null,
      rpc_endpoint: row.rpc_endpoint ? String(row.rpc_endpoint) : null,
      submitted_at: String(row.submitted_at ?? ""),
      confirmed_at: row.confirmed_at ? String(row.confirmed_at) : null,
    })),
    has_more: hasMore,
    last_id: lastId,
  };
}
