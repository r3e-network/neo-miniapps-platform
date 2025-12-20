import { handleCorsPreflight } from "../_shared/cors.ts";
import { error, json } from "../_shared/response.ts";
import { requireRateLimit } from "../_shared/ratelimit.ts";
import { requireHostScope } from "../_shared/scopes.ts";
import { ensureUserRow, requireAuth, requirePrimaryWallet, supabaseServiceClient } from "../_shared/supabase.ts";

type DeleteSecretRequest = {
  name: string;
};

// Deletes a secret and its permissions for the authenticated user.
export async function handler(req: Request): Promise<Response> {
  const preflight = handleCorsPreflight(req);
  if (preflight) return preflight;
  if (req.method !== "POST") return error(405, "method not allowed", "METHOD_NOT_ALLOWED", req);

  const auth = await requireAuth(req);
  if (auth instanceof Response) return auth;
  const rl = await requireRateLimit(req, "secrets-delete", auth);
  if (rl) return rl;
  const scopeCheck = requireHostScope(req, auth, "secrets-delete");
  if (scopeCheck) return scopeCheck;
  const walletCheck = await requirePrimaryWallet(auth.userId, req);
  if (walletCheck instanceof Response) return walletCheck;

  const ensured = await ensureUserRow(auth, {}, req);
  if (ensured instanceof Response) return ensured;

  let body: DeleteSecretRequest;
  try {
    body = await req.json();
  } catch {
    return error(400, "invalid JSON body", "BAD_JSON", req);
  }

  const name = String(body.name ?? "").trim();
  if (!name) return error(400, "name required", "NAME_REQUIRED", req);

  const supabase = supabaseServiceClient();

  // Best-effort: drop policies first.
  await supabase.from("secret_policies").delete().eq("user_id", auth.userId).eq("secret_name", name);

  const { error: delErr, count } = await supabase
    .from("secrets")
    .delete({ count: "exact" })
    .eq("user_id", auth.userId)
    .eq("name", name);

  if (delErr) return error(500, `failed to delete secret: ${delErr.message}`, "DB_ERROR", req);
  if (!count) return error(404, "secret not found", "NOT_FOUND", req);

  return json({ status: "ok" }, {}, req);
}

if (import.meta.main) {
  Deno.serve(handler);
}
