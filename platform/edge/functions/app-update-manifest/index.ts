import { handleCorsPreflight } from "../_shared/cors.ts";
import { normalizeUInt160 } from "../_shared/contracts.ts";
import { mustGetEnv } from "../_shared/env.ts";
import { computeManifestHashHex, enforceMiniAppAssetPolicy } from "../_shared/manifest.ts";
import { error, json } from "../_shared/response.ts";
import { requireAuth, requirePrimaryWallet } from "../_shared/supabase.ts";

type AppUpdateManifestRequest = {
  manifest: unknown;
};

// Thin gateway:
// - validates auth + wallet binding + shape
// - enforces manifest policy (assets_allowed=["GAS"], governance_assets_allowed=["NEO"])
// - computes the manifest hash deterministically
// - returns an invocation "intent" for the SDK/wallet to sign and submit
Deno.serve(async (req) => {
  const preflight = handleCorsPreflight(req);
  if (preflight) return preflight;
  if (req.method !== "POST") return error(405, "method not allowed", "METHOD_NOT_ALLOWED");

  const auth = await requireAuth(req);
  if (auth instanceof Response) return auth;
  const walletCheck = await requirePrimaryWallet(auth.userId);
  if (walletCheck instanceof Response) return walletCheck;

  let body: AppUpdateManifestRequest;
  try {
    body = await req.json();
  } catch {
    return error(400, "invalid JSON body", "BAD_JSON");
  }

  const manifest = (body as any)?.manifest;
  if (!manifest) return error(400, "manifest required", "MANIFEST_REQUIRED");

  let appId = "";
  let entryUrl = "";
  let manifestHash = "";
  try {
    enforceMiniAppAssetPolicy(manifest);
    manifestHash = await computeManifestHashHex(manifest);
    const m = manifest as any;
    appId = String(m?.app_id ?? "").trim();
    entryUrl = String(m?.entry_url ?? "").trim();
    if (!appId) throw new Error("manifest.app_id required");
    if (!entryUrl) throw new Error("manifest.entry_url required");
  } catch (e) {
    return error(400, (e as Error).message, "BAD_INPUT");
  }

  const appRegistryHash = normalizeUInt160(mustGetEnv("CONTRACT_APPREGISTRY_HASH"));
  const requestId = crypto.randomUUID();

  return json({
    request_id: requestId,
    user_id: auth.userId,
    intent: "apps",
    manifest_hash: manifestHash,
    invocation: {
      contract_hash: appRegistryHash,
      method: "updateManifest",
      params: [
        { type: "String", value: appId },
        { type: "ByteArray", value: manifestHash },
        { type: "String", value: entryUrl },
      ],
    },
  });
});
