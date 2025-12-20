import { handleCorsPreflight } from "../_shared/cors.ts";
import { mustGetEnv } from "../_shared/env.ts";
import { error, json } from "../_shared/response.ts";
import { requireRateLimit } from "../_shared/ratelimit.ts";
import { requireHostScope } from "../_shared/scopes.ts";
import { requireAuth, requirePrimaryWallet } from "../_shared/supabase.ts";
import { postJSON } from "../_shared/tee.ts";

type TriggerIDRequest = {
  id: string;
};

// Thin gateway to the NeoFlow service (/triggers/{id}/resume).
export async function handler(req: Request): Promise<Response> {
  const preflight = handleCorsPreflight(req);
  if (preflight) return preflight;
  if (req.method !== "POST") return error(405, "method not allowed", "METHOD_NOT_ALLOWED", req);

  const auth = await requireAuth(req);
  if (auth instanceof Response) return auth;
  const rl = await requireRateLimit(req, "automation-trigger-resume", auth);
  if (rl) return rl;
  const scopeCheck = requireHostScope(req, auth, "automation-trigger-resume");
  if (scopeCheck) return scopeCheck;
  const walletCheck = await requirePrimaryWallet(auth.userId, req);
  if (walletCheck instanceof Response) return walletCheck;

  let body: TriggerIDRequest;
  try {
    body = await req.json();
  } catch {
    return error(400, "invalid JSON body", "BAD_JSON", req);
  }

  const triggerId = String((body as any)?.id ?? "").trim();
  if (!triggerId) return error(400, "id required", "ID_REQUIRED", req);

  const neoflowURL = mustGetEnv("NEOFLOW_URL").replace(/\/$/, "");
  const result = await postJSON(`${neoflowURL}/triggers/${encodeURIComponent(triggerId)}/resume`, {}, {
    "X-User-ID": auth.userId,
  }, req);
  if (result instanceof Response) return result;
  return json(result, {}, req);
}

if (import.meta.main) {
  Deno.serve(handler);
}
