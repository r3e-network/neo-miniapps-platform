import { useCallback, useState } from "react";
import {
  Datafeed,
  DatafeedUpdate,
  DatastreamFrame,
  PriceSnapshot,
  createDatalinkChannel,
  createDatalinkDelivery,
  fetchDatafeedUpdates,
  fetchDatafeeds,
  fetchDatalinkChannels,
  fetchDatalinkDeliveries,
  fetchDatastreamFrames,
  fetchDatastreams,
  fetchDTAOrders,
  fetchDTAProducts,
  fetchPriceFeeds,
  fetchPriceSnapshots,
  updateDatafeedAggregation,
} from "../api";
import { DatafeedsState } from "../components/DatafeedsPanel";
import { DatalinkState } from "../components/DatalinkPanel";
import { DatastreamsState } from "../components/DatastreamsPanel";
import { DTAState } from "../components/DTAPanel";
import { PricefeedsState } from "../components/PricefeedsPanel";

export function useFeedsResources(config: { baseUrl: string; token: string; tenant?: string }) {
  const [datafeeds, setDatafeeds] = useState<Record<string, DatafeedsState>>({});
  const [pricefeeds, setPricefeeds] = useState<Record<string, PricefeedsState>>({});
  const [datalink, setDatalink] = useState<Record<string, DatalinkState>>({});
  const [datastreams, setDatastreams] = useState<Record<string, DatastreamsState>>({});
  const [dta, setDTA] = useState<Record<string, DTAState>>({});

  const resetFeeds = useCallback(() => {
    setDatafeeds({});
    setPricefeeds({});
    setDatalink({});
    setDatastreams({});
    setDTA({});
  }, []);

  const loadDatafeeds = useCallback(
    async (accountID: string) => {
      setDatafeeds((prev) => ({ ...prev, [accountID]: { status: "loading" } }));
      try {
        const feeds = await fetchDatafeeds(config, accountID);
        const updatesEntries = await Promise.all(
          feeds.map(async (feed): Promise<[string, DatafeedUpdate[]]> => {
            try {
              const resp = await fetchDatafeedUpdates(config, accountID, feed.ID, 5);
              return [feed.ID, resp];
            } catch {
              return [feed.ID, []];
            }
          }),
        );
        const updates = updatesEntries.reduce<Record<string, DatafeedUpdate[]>>((acc, [feedID, rows]) => {
          acc[feedID] = rows;
          return acc;
        }, {});
        setDatafeeds((prev) => ({ ...prev, [accountID]: { status: "ready", feeds, updates } }));
      } catch (err) {
        const message = err instanceof Error ? err.message : String(err);
        setDatafeeds((prev) => ({ ...prev, [accountID]: { status: "error", message } }));
      }
    },
    [config],
  );

  const loadPricefeeds = useCallback(
    async (accountID: string) => {
      setPricefeeds((prev) => ({ ...prev, [accountID]: { status: "loading" } }));
      try {
        const feeds = await fetchPriceFeeds(config, accountID);
        const snapshots: Record<string, PriceSnapshot[]> = {};
        for (const feed of feeds) {
          snapshots[feed.ID] = await fetchPriceSnapshots(config, accountID, feed.ID, 5);
        }
        setPricefeeds((prev) => ({ ...prev, [accountID]: { status: "ready", feeds, snapshots } }));
      } catch (err) {
        const message = err instanceof Error ? err.message : String(err);
        setPricefeeds((prev) => ({ ...prev, [accountID]: { status: "error", message } }));
      }
    },
    [config],
  );

  const loadDatalink = useCallback(
    async (accountID: string) => {
      setDatalink((prev) => ({ ...prev, [accountID]: { status: "loading" } }));
      try {
        const [channels, deliveries] = await Promise.all([fetchDatalinkChannels(config, accountID), fetchDatalinkDeliveries(config, accountID)]);
        setDatalink((prev) => ({ ...prev, [accountID]: { status: "ready", channels, deliveries } }));
      } catch (err) {
        const message = err instanceof Error ? err.message : String(err);
        setDatalink((prev) => ({ ...prev, [accountID]: { status: "error", message } }));
      }
    },
    [config],
  );

  const loadDatastreams = useCallback(
    async (accountID: string) => {
      setDatastreams((prev) => ({ ...prev, [accountID]: { status: "loading" } }));
      try {
        const streams = await fetchDatastreams(config, accountID);
        const frames: Record<string, DatastreamFrame[]> = {};
        for (const stream of streams) {
          frames[stream.ID] = await fetchDatastreamFrames(config, accountID, stream.ID, 5);
        }
        setDatastreams((prev) => ({ ...prev, [accountID]: { status: "ready", streams, frames } }));
      } catch (err) {
        const message = err instanceof Error ? err.message : String(err);
        setDatastreams((prev) => ({ ...prev, [accountID]: { status: "error", message } }));
      }
    },
    [config],
  );

  const loadDTA = useCallback(
    async (accountID: string) => {
      setDTA((prev) => ({ ...prev, [accountID]: { status: "loading" } }));
      try {
        const [products, orders] = await Promise.all([fetchDTAProducts(config, accountID), fetchDTAOrders(config, accountID)]);
        setDTA((prev) => ({ ...prev, [accountID]: { status: "ready", products, orders } }));
      } catch (err) {
        const message = err instanceof Error ? err.message : String(err);
        setDTA((prev) => ({ ...prev, [accountID]: { status: "error", message } }));
      }
    },
    [config],
  );

  const setAggregation = useCallback(
    async (accountID: string, feed: Datafeed, aggregation: string) => {
      await updateDatafeedAggregation(config, accountID, feed, aggregation);
      await loadDatafeeds(accountID);
    },
    [config, loadDatafeeds],
  );

  const createChannel = useCallback(
    async (accountID: string, payload: { name: string; endpoint: string; signers: string[]; status?: string; metadata?: Record<string, string> }) => {
      await createDatalinkChannel(config, accountID, payload);
      await loadDatalink(accountID);
    },
    [config, loadDatalink],
  );

  const createDelivery = useCallback(
    async (accountID: string, channelID: string, payload: { body: Record<string, any>; metadata?: Record<string, string> }) => {
      await createDatalinkDelivery(config, accountID, channelID, payload);
      await loadDatalink(accountID);
    },
    [config, loadDatalink],
  );

  return {
    datafeeds,
    pricefeeds,
    datalink,
    datastreams,
    dta,
    loadDatafeeds,
    loadPricefeeds,
    loadDatalink,
    loadDatastreams,
    loadDTA,
    setAggregation,
    createChannel,
    createDelivery,
    resetFeeds,
  };
}
