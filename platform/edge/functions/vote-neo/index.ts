import { handleCorsPreflight } from "../_shared/cors.ts";
import { normalizeUInt160 } from "../_shared/contracts.ts";
import { mustGetEnv } from "../_shared/env.ts";
import { error, json } from "../_shared/response.ts";
import { requireRateLimit } from "../_shared/ratelimit.ts";
import { requireScope } from "../_shared/scopes.ts";
import { requireAuth, requirePrimaryWallet } from "../_shared/supabase.ts";
import { enforceUsageCaps, fetchMiniAppPolicy, permissionEnabled } from "../_shared/apps.ts";

type VoteNeoRequest = {
  app_id: string;
  proposal_id: string;
  neo_amount: string;
  support?: boolean;
};

// Thin gateway:
// - validates auth + basic shape
// - enforces NEO-only governance
// - returns an invocation "intent" for the SDK/wallet to sign and submit
export async function handler(req: Request): Promise<Response> {
  const preflight = handleCorsPreflight(req);
  if (preflight) return preflight;
  if (req.method !== "POST") return error(405, "method not allowed", "METHOD_NOT_ALLOWED", req);

  const auth = await requireAuth(req);
  if (auth instanceof Response) return auth;
  const rl = await requireRateLimit(req, "vote-neo", auth);
  if (rl) return rl;
  const scopeCheck = requireScope(req, auth, "vote-neo");
  if (scopeCheck) return scopeCheck;
  const walletCheck = await requirePrimaryWallet(auth.userId, req);
  if (walletCheck instanceof Response) return walletCheck;

  let body: VoteNeoRequest;
  try {
    body = await req.json();
  } catch {
    return error(400, "invalid JSON body", "BAD_JSON", req);
  }

  const appId = (body.app_id ?? "").trim();
  if (!appId) return error(400, "app_id required", "APP_ID_REQUIRED", req);

  const policy = await fetchMiniAppPolicy(appId, req);
  if (policy instanceof Response) return policy;
  if (policy && !permissionEnabled(policy.permissions, "governance")) {
    return error(403, "app is not allowed to request governance", "PERMISSION_DENIED", req);
  }

  const proposalId = String(body.proposal_id ?? "").trim();
  if (!proposalId) return error(400, "proposal_id required", "PROPOSAL_ID_REQUIRED", req);

  const support = body.support ?? true;

  const amountStr = String(body.neo_amount ?? "").trim();
  if (!/^\d+$/.test(amountStr)) return error(400, "neo_amount must be an integer string", "AMOUNT_INVALID", req);
  const amount = BigInt(amountStr);
  if (amount <= 0n) return error(400, "neo_amount must be > 0", "AMOUNT_INVALID", req);
  if (policy?.limits.governanceCap && amount > policy.limits.governanceCap) {
    return error(403, "neo_amount exceeds manifest limit", "LIMIT_EXCEEDED", req);
  }
  if (policy?.limits.governanceCap) {
    const usageErr = await enforceUsageCaps({
      appId,
      userId: auth.userId,
      governanceDelta: amount,
      governanceCap: policy.limits.governanceCap,
      req,
    });
    if (usageErr) return usageErr;
  }

  const governanceHash = normalizeUInt160(mustGetEnv("CONTRACT_GOVERNANCE_HASH"));

  const requestId = crypto.randomUUID();

  return json({
    request_id: requestId,
    user_id: auth.userId,
    intent: "governance",
    constraints: { governance: "NEO_ONLY" },
    invocation: {
      contract_hash: governanceHash,
      method: "vote",
      params: [
        { type: "String", value: proposalId },
        { type: "Boolean", value: support },
        { type: "Integer", value: amount.toString() },
      ],
    },
  }, {}, req);
}

if (import.meta.main) {
  Deno.serve(handler);
}
