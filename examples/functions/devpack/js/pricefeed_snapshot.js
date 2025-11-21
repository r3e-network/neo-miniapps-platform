// Record a price snapshot for a feed using Devpack runtime.
// Expects params.feedId and params.price; optional params.source/collectedAt.
export default function (params) {
  Devpack.priceFeeds.recordSnapshot({
    feedId: params.feedId,
    price: params.price,
    source: params.source || "manual",
    collectedAt: params.collectedAt,
  });

  return {
    success: true,
    data: {
      feedId: params.feedId,
      price: params.price,
      collectedAt: params.collectedAt || new Date().toISOString(),
    },
    meta: null,
  };
}
