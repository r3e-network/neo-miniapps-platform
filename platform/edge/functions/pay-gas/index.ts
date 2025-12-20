import { handleCorsPreflight } from "../_shared/cors.ts";
import { normalizeUInt160 } from "../_shared/contracts.ts";
import { mustGetEnv } from "../_shared/env.ts";
import { parseDecimalToInt } from "../_shared/amount.ts";
import { error, json } from "../_shared/response.ts";
import { requireRateLimit } from "../_shared/ratelimit.ts";
import { requireScope } from "../_shared/scopes.ts";
import { requireAuth, requirePrimaryWallet } from "../_shared/supabase.ts";
import { enforceUsageCaps, fetchMiniAppPolicy, permissionEnabled } from "../_shared/apps.ts";

type PayGasRequest = {
  app_id: string;
  amount_gas: string;
  memo?: string;
};

// Thin gateway:
// - validates auth + basic shape
// - enforces GAS-only settlement
// - returns an invocation "intent" for the SDK/wallet to sign and submit
export async function handler(req: Request): Promise<Response> {
  const preflight = handleCorsPreflight(req);
  if (preflight) return preflight;
  if (req.method !== "POST") return error(405, "method not allowed", "METHOD_NOT_ALLOWED", req);

  const auth = await requireAuth(req);
  if (auth instanceof Response) return auth;
  const rl = await requireRateLimit(req, "pay-gas", auth);
  if (rl) return rl;
  const scopeCheck = requireScope(req, auth, "pay-gas");
  if (scopeCheck) return scopeCheck;
  const walletCheck = await requirePrimaryWallet(auth.userId, req);
  if (walletCheck instanceof Response) return walletCheck;

  let body: PayGasRequest;
  try {
    body = await req.json();
  } catch {
    return error(400, "invalid JSON body", "BAD_JSON", req);
  }

  const appId = (body.app_id ?? "").trim();
  if (!appId) return error(400, "app_id required", "APP_ID_REQUIRED", req);

  const policy = await fetchMiniAppPolicy(appId, req);
  if (policy instanceof Response) return policy;
  if (policy && !permissionEnabled(policy.permissions, "payments")) {
    return error(403, "app is not allowed to request payments", "PERMISSION_DENIED", req);
  }

  let amount;
  try {
    amount = parseDecimalToInt(String(body.amount_gas ?? ""), 8);
  } catch (e) {
    return error(400, `amount_gas invalid: ${(e as Error).message}`, "AMOUNT_INVALID", req);
  }
  if (amount <= 0n) return error(400, "amount_gas must be > 0", "AMOUNT_INVALID", req);
  if (policy?.limits.maxGasPerTx && amount > policy.limits.maxGasPerTx) {
    return error(403, "amount_gas exceeds manifest limit", "LIMIT_EXCEEDED", req);
  }
  if (policy?.limits.dailyGasCapPerUser) {
    const usageErr = await enforceUsageCaps({
      appId,
      userId: auth.userId,
      gasDelta: amount,
      gasCap: policy.limits.dailyGasCapPerUser,
      req,
    });
    if (usageErr) return usageErr;
  }

  const paymentHubHash = normalizeUInt160(mustGetEnv("CONTRACT_PAYMENTHUB_HASH"));

  const requestId = crypto.randomUUID();
  const memo = (body.memo ?? "").slice(0, 256);

  return json({
    request_id: requestId,
    user_id: auth.userId,
    intent: "payments",
    constraints: { settlement: "GAS_ONLY" },
    invocation: {
      contract_hash: paymentHubHash,
      method: "pay",
      params: [
        { type: "String", value: appId },
        { type: "Integer", value: amount.toString() },
        { type: "String", value: memo },
      ],
    },
  }, {}, req);
}

if (import.meta.main) {
  Deno.serve(handler);
}
