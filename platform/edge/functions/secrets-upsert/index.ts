import { handleCorsPreflight } from "../_shared/cors.ts";
import { error, json } from "../_shared/response.ts";
import { requireRateLimit } from "../_shared/ratelimit.ts";
import { encryptSecretValue } from "../_shared/secrets.ts";
import { requireHostScope } from "../_shared/scopes.ts";
import { ensureUserRow, requireAuth, requirePrimaryWallet, supabaseServiceClient } from "../_shared/supabase.ts";

type UpsertSecretRequest = {
  name: string;
  value: string;
};

const MAX_NAME_LEN = 128;
const MAX_VALUE_BYTES = 64 * 1024;

// Creates or updates a secret for the authenticated user.
// Values are AES-GCM encrypted with `SECRETS_MASTER_KEY` before storage.
export async function handler(req: Request): Promise<Response> {
  const preflight = handleCorsPreflight(req);
  if (preflight) return preflight;
  if (req.method !== "POST") return error(405, "method not allowed", "METHOD_NOT_ALLOWED", req);

  const auth = await requireAuth(req);
  if (auth instanceof Response) return auth;
  const rl = await requireRateLimit(req, "secrets-upsert", auth);
  if (rl) return rl;
  const scopeCheck = requireHostScope(req, auth, "secrets-upsert");
  if (scopeCheck) return scopeCheck;
  const walletCheck = await requirePrimaryWallet(auth.userId, req);
  if (walletCheck instanceof Response) return walletCheck;

  const ensured = await ensureUserRow(auth, {}, req);
  if (ensured instanceof Response) return ensured;

  let body: UpsertSecretRequest;
  try {
    body = await req.json();
  } catch {
    return error(400, "invalid JSON body", "BAD_JSON", req);
  }

  const name = String(body.name ?? "").trim();
  const value = String(body.value ?? "");
  if (!name) return error(400, "name required", "NAME_REQUIRED", req);
  if (!value) return error(400, "value required", "VALUE_REQUIRED", req);
  if (name.length > MAX_NAME_LEN) return error(400, "name too long", "NAME_INVALID", req);
  if (new TextEncoder().encode(value).length > MAX_VALUE_BYTES) {
    return error(400, "value too large", "VALUE_INVALID", req);
  }

  const encryptedBase64 = await encryptSecretValue(value);

  const supabase = supabaseServiceClient();

  const { data: existing, error: getErr } = await supabase
    .from("secrets")
    .select("id,version")
    .eq("user_id", auth.userId)
    .eq("name", name)
    .limit(1);
  if (getErr) return error(500, `failed to load secret: ${getErr.message}`, "DB_ERROR", req);

  const isCreate = !existing || existing.length === 0;

  if (isCreate) {
    const { data: inserted, error: insertErr } = await supabase
      .from("secrets")
      .insert({
        user_id: auth.userId,
        name,
        encrypted_value: encryptedBase64,
        version: 1,
      })
      .select("id,name,version,created_at,updated_at")
      .maybeSingle();

    if (insertErr) return error(500, `failed to create secret: ${insertErr.message}`, "DB_ERROR", req);

    // Reset permissions on create (best-effort).
    await supabase.from("secret_policies").delete().eq("user_id", auth.userId).eq("secret_name", name);

    return json({ secret: inserted, created: true }, {}, req);
  }

  const currentVersion = Number((existing[0] as any)?.version ?? 0);
  const nextVersion = Number.isFinite(currentVersion) && currentVersion > 0 ? currentVersion + 1 : 1;

  const { data: updated, error: updateErr } = await supabase
    .from("secrets")
    .update({ encrypted_value: encryptedBase64, version: nextVersion })
    .eq("user_id", auth.userId)
    .eq("name", name)
    .select("id,name,version,created_at,updated_at")
    .maybeSingle();

  if (updateErr) return error(500, `failed to update secret: ${updateErr.message}`, "DB_ERROR", req);

  return json({ secret: updated, created: false }, {}, req);
}

if (import.meta.main) {
  Deno.serve(handler);
}
