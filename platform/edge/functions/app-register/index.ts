import { handleCorsPreflight } from "../_shared/cors.ts";
import { normalizeUInt160 } from "../_shared/contracts.ts";
import { mustGetEnv } from "../_shared/env.ts";
import { error, json } from "../_shared/response.ts";
import { requireAuth, requirePrimaryWallet } from "../_shared/supabase.ts";

type AppRegisterRequest = {
  app_id: string;
  manifest_hash: string;
  entry_url: string;
  developer_pubkey: string;
};

function normalizeHexBytes(value: string, expectedBytes: number, label: string): string {
  let s = String(value ?? "").trim();
  s = s.replace(/^0x/i, "");
  if (!s) throw new Error(`${label} required`);
  if (!/^[0-9a-fA-F]+$/.test(s)) throw new Error(`${label} must be hex`);
  if (s.length !== expectedBytes * 2) throw new Error(`${label} must be ${expectedBytes} bytes`);
  return s.toLowerCase();
}

// Thin gateway:
// - validates auth + wallet binding + shape
// - returns an invocation "intent" for the SDK/wallet to sign and submit
Deno.serve(async (req) => {
  const preflight = handleCorsPreflight(req);
  if (preflight) return preflight;
  if (req.method !== "POST") return error(405, "method not allowed", "METHOD_NOT_ALLOWED");

  const auth = await requireAuth(req);
  if (auth instanceof Response) return auth;
  const walletCheck = await requirePrimaryWallet(auth.userId);
  if (walletCheck instanceof Response) return walletCheck;

  let body: AppRegisterRequest;
  try {
    body = await req.json();
  } catch {
    return error(400, "invalid JSON body", "BAD_JSON");
  }

  const appId = String(body.app_id ?? "").trim();
  if (!appId) return error(400, "app_id required", "APP_ID_REQUIRED");

  const entryUrl = String(body.entry_url ?? "").trim();
  if (!entryUrl) return error(400, "entry_url required", "ENTRY_URL_REQUIRED");

  let manifestHash: string;
  let developerPubKey: string;
  try {
    manifestHash = normalizeHexBytes(body.manifest_hash, 32, "manifest_hash");
    developerPubKey = normalizeHexBytes(body.developer_pubkey, 33, "developer_pubkey");
  } catch (e) {
    return error(400, (e as Error).message, "BAD_INPUT");
  }

  const appRegistryHash = normalizeUInt160(mustGetEnv("CONTRACT_APPREGISTRY_HASH"));
  const requestId = crypto.randomUUID();

  return json({
    request_id: requestId,
    user_id: auth.userId,
    intent: "apps",
    invocation: {
      contract_hash: appRegistryHash,
      method: "register",
      params: [
        { type: "String", value: appId },
        { type: "ByteArray", value: manifestHash },
        { type: "String", value: entryUrl },
        { type: "ByteArray", value: developerPubKey },
      ],
    },
  });
});

