// Demonstrates orchestrating data feeds, data streams, and DataLink from Devpack.
// Expects:
// - params.feedId (data feed ID)
// - params.streamId (data stream ID)
// - params.channelId (DataLink channel ID)
// - params.price (string/number)
// - optional params.sequence (number), params.metadata (object)
export default function (params = {}) {
  const price = Number(params.price);
  if (!Number.isFinite(price) || price <= 0) {
    throw new Error("price must be a positive number");
  }

  const feedId = String(params.feedId || "");
  const streamId = String(params.streamId || "");
  const channelId = String(params.channelId || "");
  if (!feedId || !streamId || !channelId) {
    throw new Error("feedId, streamId, and channelId are required");
  }

  // Queue a data feed update
  const update = Devpack.dataFeeds.submitUpdate({
    feedId,
    roundId: params.roundId || 1,
    price: String(price),
    timestamp: params.timestamp,
    metadata: params.metadata,
  });

  // Publish a data stream frame
  const frame = Devpack.dataStreams.publishFrame({
    streamId,
    sequence: params.sequence || 1,
    payload: { price },
    latencyMs: params.latencyMs || 0,
    metadata: params.metadata,
  });

  // Enqueue a DataLink delivery with the same payload
  const delivery = Devpack.dataLink.createDelivery({
    channelId,
    payload: { price },
    metadata: params.metadata,
  });

  return Devpack.respond.success({
    feedId,
    streamId,
    channelId,
    price,
    actions: [
      update.asResult({ label: "datafeed_update" }),
      frame.asResult({ label: "datastream_frame" }),
      delivery.asResult({ label: "datalink_delivery" }),
    ],
  });
}
