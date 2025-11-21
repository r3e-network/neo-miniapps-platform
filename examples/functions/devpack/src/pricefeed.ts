import { recordPriceSnapshot, respond } from "@service-layer/devpack";

interface Params {
  feedId: string;
  price: number;
  source?: string;
  collectedAt?: string;
}

export default function handler(raw: Params) {
  const feedId = String(raw.feedId || "");
  const price = Number(raw.price);

  if (!feedId) {
    throw new Error("feedId is required");
  }
  if (!Number.isFinite(price) || price <= 0) {
    throw new Error("price must be a positive number");
  }

  recordPriceSnapshot({
    feedId,
    price,
    source: raw.source,
    collectedAt: raw.collectedAt,
  });

  return respond.success({
    feedId,
    price,
    collectedAt: raw.collectedAt || new Date().toISOString(),
  });
}
