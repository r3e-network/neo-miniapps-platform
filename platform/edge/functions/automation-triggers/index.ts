import { handleCorsPreflight } from "../_shared/cors.ts";
import { mustGetEnv } from "../_shared/env.ts";
import { error, json } from "../_shared/response.ts";
import { requireRateLimit } from "../_shared/ratelimit.ts";
import { requireHostScope } from "../_shared/scopes.ts";
import { requireAuth, requirePrimaryWallet } from "../_shared/supabase.ts";
import { getJSON, postJSON } from "../_shared/tee.ts";

type TriggerRequest = {
  name: string;
  trigger_type: string;
  schedule?: string;
  condition?: unknown;
  action: unknown;
};

// Thin gateway to the NeoFlow service (/triggers):
// - validates auth + wallet binding
// - forwards to the TEE service over optional mTLS
export async function handler(req: Request): Promise<Response> {
  const preflight = handleCorsPreflight(req);
  if (preflight) return preflight;

  if (req.method !== "GET" && req.method !== "POST") {
    return error(405, "method not allowed", "METHOD_NOT_ALLOWED", req);
  }

  const auth = await requireAuth(req);
  if (auth instanceof Response) return auth;
  const rl = await requireRateLimit(req, "automation-triggers", auth);
  if (rl) return rl;
  const scopeCheck = requireHostScope(req, auth, "automation-triggers");
  if (scopeCheck) return scopeCheck;
  const walletCheck = await requirePrimaryWallet(auth.userId, req);
  if (walletCheck instanceof Response) return walletCheck;

  const neoflowURL = mustGetEnv("NEOFLOW_URL").replace(/\/$/, "");

  if (req.method === "GET") {
    const result = await getJSON(`${neoflowURL}/triggers`, { "X-User-ID": auth.userId }, req);
    if (result instanceof Response) return result;
    return json(result, {}, req);
  }

  let body: TriggerRequest;
  try {
    body = await req.json();
  } catch {
    return error(400, "invalid JSON body", "BAD_JSON", req);
  }

  const name = String((body as any)?.name ?? "").trim();
  const triggerType = String((body as any)?.trigger_type ?? "").trim();
  if (!name || !triggerType) return error(400, "name and trigger_type required", "BAD_INPUT", req);
  if ((body as any)?.action === undefined || (body as any)?.action === null) {
    return error(400, "action required", "BAD_INPUT", req);
  }

  const result = await postJSON(`${neoflowURL}/triggers`, body, { "X-User-ID": auth.userId }, req);
  if (result instanceof Response) return result;
  return json(result, {}, req);
}

if (import.meta.main) {
  Deno.serve(handler);
}
