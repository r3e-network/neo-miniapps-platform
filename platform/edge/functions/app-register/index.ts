import { handleCorsPreflight } from "../_shared/cors.ts";
import { normalizeUInt160 } from "../_shared/contracts.ts";
import { mustGetEnv } from "../_shared/env.ts";
import { upsertMiniAppManifest } from "../_shared/apps.ts";
import { canonicalizeMiniAppManifest, parseMiniAppManifestCore } from "../_shared/manifest.ts";
import { error, json } from "../_shared/response.ts";
import { requireRateLimit } from "../_shared/ratelimit.ts";
import { requireScope } from "../_shared/scopes.ts";
import { requireAuth, requirePrimaryWallet } from "../_shared/supabase.ts";

type AppRegisterRequest = {
  manifest: unknown;
};

// Thin gateway:
// - validates auth + wallet binding + shape
// - enforces manifest policy (assets_allowed=["GAS"], governance_assets_allowed=["NEO"])
// - computes the manifest hash deterministically
// - returns an invocation "intent" for the SDK/wallet to sign and submit
export async function handler(req: Request): Promise<Response> {
  const preflight = handleCorsPreflight(req);
  if (preflight) return preflight;
  if (req.method !== "POST") return error(405, "method not allowed", "METHOD_NOT_ALLOWED", req);

  const auth = await requireAuth(req);
  if (auth instanceof Response) return auth;
  const rl = await requireRateLimit(req, "app-register", auth);
  if (rl) return rl;
  const scopeCheck = requireScope(req, auth, "app-register");
  if (scopeCheck) return scopeCheck;
  const walletCheck = await requirePrimaryWallet(auth.userId, req);
  if (walletCheck instanceof Response) return walletCheck;

  let body: AppRegisterRequest;
  try {
    body = await req.json();
  } catch {
    return error(400, "invalid JSON body", "BAD_JSON", req);
  }

  const manifest = (body as any)?.manifest;
  if (!manifest) return error(400, "manifest required", "MANIFEST_REQUIRED", req);

  let core;
  let canonical;
  try {
    core = await parseMiniAppManifestCore(manifest);
    canonical = canonicalizeMiniAppManifest(manifest);
  } catch (e) {
    return error(400, (e as Error).message, "BAD_INPUT", req);
  }

  const upsertErr = await upsertMiniAppManifest({
    core,
    canonicalManifest: canonical,
    developerUserId: auth.userId,
    mode: "register",
    req,
  });
  if (upsertErr) return upsertErr;

  const appRegistryHash = normalizeUInt160(mustGetEnv("CONTRACT_APPREGISTRY_HASH"));
  const requestId = crypto.randomUUID();

  return json({
    request_id: requestId,
    user_id: auth.userId,
    intent: "apps",
    manifest_hash: core.manifestHashHex,
    invocation: {
      contract_hash: appRegistryHash,
      method: "register",
      params: [
        { type: "String", value: core.appId },
        { type: "ByteArray", value: core.manifestHashHex },
        { type: "String", value: core.entryUrl },
        { type: "ByteArray", value: core.developerPubKeyHex },
      ],
    },
  }, {}, req);
}

if (import.meta.main) {
  Deno.serve(handler);
}
