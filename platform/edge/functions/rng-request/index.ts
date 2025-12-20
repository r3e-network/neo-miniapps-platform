import { handleCorsPreflight } from "../_shared/cors.ts";
import { normalizeUInt160 } from "../_shared/contracts.ts";
import { getEnv, mustGetEnv } from "../_shared/env.ts";
import { error, json } from "../_shared/response.ts";
import { requireRateLimit } from "../_shared/ratelimit.ts";
import { requireScope } from "../_shared/scopes.ts";
import { requireAuth, requirePrimaryWallet } from "../_shared/supabase.ts";
import { postJSON } from "../_shared/tee.ts";
import { fetchMiniAppPolicy, permissionEnabled } from "../_shared/apps.ts";

type RNGRequest = {
  app_id: string;
};

export async function handler(req: Request): Promise<Response> {
  const preflight = handleCorsPreflight(req);
  if (preflight) return preflight;
  if (req.method !== "POST") return error(405, "method not allowed", "METHOD_NOT_ALLOWED", req);

  const auth = await requireAuth(req);
  if (auth instanceof Response) return auth;
  const rl = await requireRateLimit(req, "rng-request", auth);
  if (rl) return rl;
  const scopeCheck = requireScope(req, auth, "rng-request");
  if (scopeCheck) return scopeCheck;
  const walletCheck = await requirePrimaryWallet(auth.userId, req);
  if (walletCheck instanceof Response) return walletCheck;

  let body: RNGRequest;
  try {
    body = await req.json();
  } catch {
    return error(400, "invalid JSON body", "BAD_JSON", req);
  }
  const appId = (body.app_id ?? "").trim();
  if (!appId) return error(400, "app_id required", "APP_ID_REQUIRED", req);

  const policy = await fetchMiniAppPolicy(appId, req);
  if (policy instanceof Response) return policy;
  if (policy) {
    const allowed = permissionEnabled(policy.permissions, "rng");
    if (!allowed) {
      return error(403, "app is not allowed to request randomness", "PERMISSION_DENIED", req);
    }
  }

  const requestId = crypto.randomUUID();

  const neovrfURL = mustGetEnv("NEOVRF_URL");
  const vrfResult = await postJSON(
    `${neovrfURL.replace(/\/$/, "")}/random`,
    { request_id: requestId },
    { "X-User-ID": auth.userId },
    req,
  );
  if (vrfResult instanceof Response) return vrfResult;

  const responseId = String((vrfResult as any)?.request_id ?? "").trim();
  if (responseId && responseId !== requestId) {
    return error(502, "vrf request_id mismatch", "RNG_REQUEST_ID_MISMATCH", req);
  }

  const randomnessHex = String((vrfResult as any)?.randomness ?? "").trim();
  const signatureHex = String((vrfResult as any)?.signature ?? "").trim();
  const publicKeyHex = String((vrfResult as any)?.public_key ?? "").trim();
  const attestationHex = String((vrfResult as any)?.attestation_hash ?? "").trim();
  if (!/^[0-9a-fA-F]+$/.test(randomnessHex) || randomnessHex.length < 2) {
    return error(502, "invalid randomness output", "RNG_INVALID_OUTPUT", req);
  }
  const attestationHash = /^[0-9a-fA-F]+$/.test(attestationHex) ? attestationHex : "";
  const signature = /^[0-9a-fA-F]+$/.test(signatureHex) ? signatureHex : "";
  const publicKey = /^[0-9a-fA-F]+$/.test(publicKeyHex) ? publicKeyHex : "";

  // Optional on-chain anchoring (RandomnessLog.record) via txproxy.
  let anchoredTx: unknown = undefined;
  if (getEnv("RNG_ANCHOR") === "1") {
    const txproxyURL = mustGetEnv("TXPROXY_URL");
    const randomnessLogHash = normalizeUInt160(mustGetEnv("CONTRACT_RANDOMNESSLOG_HASH"));
    const timestamp = Math.floor(Date.now() / 1000);
    const reportHashHex = attestationHash || randomnessHex.slice(0, 64);

    const txRes = await postJSON(
      `${txproxyURL.replace(/\/$/, "")}/invoke`,
      {
        request_id: requestId,
        contract_hash: randomnessLogHash,
        method: "record",
        params: [
          { type: "String", value: requestId },
          { type: "ByteArray", value: randomnessHex },
          { type: "ByteArray", value: reportHashHex },
          { type: "Integer", value: String(timestamp) },
        ],
        wait: true,
      },
      { "X-Service-ID": "gateway" },
      req,
    );
    if (txRes instanceof Response) return txRes;
    anchoredTx = txRes;
  }

  return json({
    request_id: requestId,
    app_id: appId,
    randomness: randomnessHex,
    signature: signature || undefined,
    public_key: publicKey || undefined,
    attestation_hash: attestationHash || undefined,
    anchored_tx: anchoredTx,
  }, {}, req);
}

if (import.meta.main) {
  Deno.serve(handler);
}
