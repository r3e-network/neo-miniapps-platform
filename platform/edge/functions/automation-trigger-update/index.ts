import { handleCorsPreflight } from "../_shared/cors.ts";
import { mustGetEnv } from "../_shared/env.ts";
import { error, json } from "../_shared/response.ts";
import { requireRateLimit } from "../_shared/ratelimit.ts";
import { requireHostScope } from "../_shared/scopes.ts";
import { requireAuth, requirePrimaryWallet } from "../_shared/supabase.ts";
import { requestJSON } from "../_shared/tee.ts";

type UpdateTriggerRequest = {
  id: string;
  name: string;
  trigger_type: string;
  schedule?: string;
  condition?: unknown;
  action: unknown;
};

// Thin gateway to the NeoFlow service (PUT /triggers/{id}).
// Uses POST in Edge for portability.
export async function handler(req: Request): Promise<Response> {
  const preflight = handleCorsPreflight(req);
  if (preflight) return preflight;
  if (req.method !== "POST") return error(405, "method not allowed", "METHOD_NOT_ALLOWED", req);

  const auth = await requireAuth(req);
  if (auth instanceof Response) return auth;
  const rl = await requireRateLimit(req, "automation-trigger-update", auth);
  if (rl) return rl;
  const scopeCheck = requireHostScope(req, auth, "automation-trigger-update");
  if (scopeCheck) return scopeCheck;
  const walletCheck = await requirePrimaryWallet(auth.userId, req);
  if (walletCheck instanceof Response) return walletCheck;

  let body: UpdateTriggerRequest;
  try {
    body = await req.json();
  } catch {
    return error(400, "invalid JSON body", "BAD_JSON", req);
  }

  const triggerId = String((body as any)?.id ?? "").trim();
  if (!triggerId) return error(400, "id required", "ID_REQUIRED", req);

  const name = String((body as any)?.name ?? "").trim();
  const triggerType = String((body as any)?.trigger_type ?? "").trim();
  if (!name || !triggerType) return error(400, "name and trigger_type required", "BAD_INPUT", req);
  if ((body as any)?.action === undefined || (body as any)?.action === null) {
    return error(400, "action required", "BAD_INPUT", req);
  }

  const neoflowURL = mustGetEnv("NEOFLOW_URL").replace(/\/$/, "");
  const result = await requestJSON(`${neoflowURL}/triggers/${encodeURIComponent(triggerId)}`, {
    method: "PUT",
    headers: { "X-User-ID": auth.userId },
    body: {
      name,
      trigger_type: triggerType,
      schedule: (body as any)?.schedule,
      condition: (body as any)?.condition,
      action: (body as any)?.action,
    },
  }, req);
  if (result instanceof Response) return result;
  return json(result, {}, req);
}

if (import.meta.main) {
  Deno.serve(handler);
}
