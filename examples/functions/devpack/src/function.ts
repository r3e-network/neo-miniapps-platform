import {
  ensureGasAccount,
  createOracleRequest,
  respond,
  currentContext,
} from "@service-layer/devpack";

interface Params {
  wallet?: string;
  oracleSourceId: string;
  pair: string;
}

export default function handler(rawParams: Params) {
  const params: Params = {
    wallet: rawParams.wallet ?? "",
    oracleSourceId: rawParams.oracleSourceId,
    pair: rawParams.pair,
  };

  ensureGasAccount({ wallet: params.wallet });

  createOracleRequest({
    dataSourceId: params.oracleSourceId,
    payload: { pair: params.pair, requestedBy: currentContext().accountId },
  });

  return respond.success({
    pair: params.pair,
    submittedAt: new Date().toISOString(),
  });
}

