import { handleCorsPreflight } from "../_shared/cors.ts";
import { error, json } from "../_shared/response.ts";
import { requireRateLimit } from "../_shared/ratelimit.ts";
import { requireHostScope } from "../_shared/scopes.ts";
import { ensureUserRow, requireAuth, requirePrimaryWallet, supabaseServiceClient } from "../_shared/supabase.ts";

// Lists secret metadata for the authenticated user (no values).
export async function handler(req: Request): Promise<Response> {
  const preflight = handleCorsPreflight(req);
  if (preflight) return preflight;
  if (req.method !== "GET") return error(405, "method not allowed", "METHOD_NOT_ALLOWED", req);

  const auth = await requireAuth(req);
  if (auth instanceof Response) return auth;
  const rl = await requireRateLimit(req, "secrets-list", auth);
  if (rl) return rl;
  const scopeCheck = requireHostScope(req, auth, "secrets-list");
  if (scopeCheck) return scopeCheck;
  const walletCheck = await requirePrimaryWallet(auth.userId, req);
  if (walletCheck instanceof Response) return walletCheck;

  const ensured = await ensureUserRow(auth, {}, req);
  if (ensured instanceof Response) return ensured;

  const supabase = supabaseServiceClient();
  const { data, error: listErr } = await supabase
    .from("secrets")
    .select("id,name,version,created_at,updated_at")
    .eq("user_id", auth.userId)
    .order("updated_at", { ascending: false });

  if (listErr) return error(500, `failed to list secrets: ${listErr.message}`, "DB_ERROR", req);
  return json({ secrets: data ?? [] }, {}, req);
}

if (import.meta.main) {
  Deno.serve(handler);
}
