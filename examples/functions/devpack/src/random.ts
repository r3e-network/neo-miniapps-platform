import { generateRandom, respond } from "@service-layer/devpack";

interface Params {
  length?: number;
  requestId?: string;
}

export default function handler(raw: Params) {
  const length = raw.length ?? 32;
  if (!Number.isFinite(length) || length <= 0 || length > 1024) {
    throw new Error("length must be between 1 and 1024");
  }

  const action = generateRandom({
    length,
    requestId: raw.requestId,
  });

  return respond.success({
    length,
    requestId: raw.requestId ?? null,
    action: action.asResult({ label: "random_generate" }),
  });
}
