using Neo;
using Neo.SmartContract;
using Neo.SmartContract.Framework;
using Neo.SmartContract.Framework.Attributes;
using Neo.SmartContract.Framework.Native;
using Neo.SmartContract.Framework.Services;
using System;
using System.ComponentModel;
using System.Numerics;

namespace ServiceLayer.DataStreams
{
    /// <summary>
    /// DataStreamsService - Real-time streaming contract.
    ///
    /// Provides low-latency, high-throughput data streaming capabilities.
    ///
    /// Flow:
    /// 1. User subscribes to data stream via Gateway
    /// 2. Service Layer (TEE) publishes real-time data updates
    /// 3. Subscribers receive data with cryptographic proofs
    ///
    /// Supports multiple stream types with quality-of-service guarantees.
    /// </summary>
    [DisplayName("DataStreamsService")]
    [ManifestExtra("Author", "R3E Network")]
    [ManifestExtra("Description", "DataStreams Service - Real-time data streaming")]
    [ContractPermission("*", "*")]
    public class DataStreamsService : SmartContract
    {
        // ==================== Storage Prefixes ====================
        private const byte PREFIX_ADMIN = 0x01;
        private const byte PREFIX_GATEWAY = 0x02;
        private const byte PREFIX_STREAM = 0x10;
        private const byte PREFIX_SUBSCRIPTION = 0x11;
        private const byte PREFIX_REPORT = 0x12;
        private const byte PREFIX_STREAM_COUNT = 0x20;
        private const byte PREFIX_PAUSED = 0x30;

        [InitialValue("NZ8zBpRh9zLaQ5DdUz4qUb9bf5MZvELyq6", ContractParameterType.Hash160)]
        private static readonly UInt160 InitialAdmin = default;

        // ==================== Events ====================

        /// <summary>
        /// Emitted when a stream is created.
        /// </summary>
        [DisplayName("StreamCreated")]
        public static event Action<ByteString, UInt160, string, BigInteger> OnStreamCreated;
        // Parameters: streamId, owner, streamType, updateInterval

        /// <summary>
        /// Emitted when a subscription is created.
        /// </summary>
        [DisplayName("StreamSubscribed")]
        public static event Action<ByteString, ByteString, UInt160> OnStreamSubscribed;
        // Parameters: subscriptionId, streamId, subscriber

        /// <summary>
        /// Emitted when stream data is published.
        /// </summary>
        [DisplayName("StreamDataPublished")]
        public static event Action<ByteString, BigInteger, ByteString, ByteString> OnStreamDataPublished;
        // Parameters: streamId, timestamp, dataHash, proof

        // ==================== Admin Methods ====================

        public static UInt160 GetAdmin()
        {
            var stored = Storage.Get(Storage.CurrentContext, new byte[] { PREFIX_ADMIN });
            return stored != null ? (UInt160)stored : InitialAdmin;
        }

        public static void SetAdmin(UInt160 newAdmin)
        {
            RequireAdmin();
            Storage.Put(Storage.CurrentContext, new byte[] { PREFIX_ADMIN }, newAdmin);
        }

        public static UInt160 GetGateway()
        {
            var stored = Storage.Get(Storage.CurrentContext, new byte[] { PREFIX_GATEWAY });
            return stored != null ? (UInt160)stored : UInt160.Zero;
        }

        public static void SetGateway(UInt160 gateway)
        {
            RequireAdmin();
            Storage.Put(Storage.CurrentContext, new byte[] { PREFIX_GATEWAY }, gateway);
        }

        public static bool IsPaused()
        {
            var stored = Storage.Get(Storage.CurrentContext, new byte[] { PREFIX_PAUSED });
            return stored != null && (BigInteger)stored == 1;
        }

        public static void Pause() { RequireAdmin(); Storage.Put(Storage.CurrentContext, new byte[] { PREFIX_PAUSED }, 1); }
        public static void Unpause() { RequireAdmin(); Storage.Delete(Storage.CurrentContext, new byte[] { PREFIX_PAUSED }); }

        // ==================== Stream Management ====================

        /// <summary>
        /// Creates a data stream. Called via Gateway.
        /// </summary>
        public static void CreateStream(ByteString streamId, UInt160 owner, string streamType, BigInteger updateInterval, ByteString config)
        {
            RequireGateway();
            RequireNotPaused();

            // Validate stream type
            if (streamType != "price" && streamType != "event" && streamType != "metric")
                throw new Exception("Invalid stream type");

            // Check if stream already exists
            if (GetStream(streamId) != null)
                throw new Exception("Stream already exists");

            var stream = new DataStream
            {
                StreamId = streamId,
                Owner = owner,
                StreamType = streamType,
                UpdateInterval = updateInterval,
                Config = config,
                CreatedAt = Runtime.Time,
                IsActive = true,
                LastUpdateAt = 0
            };
            StoreStream(streamId, stream);

            IncrementStreamCount();

            OnStreamCreated(streamId, owner, streamType, updateInterval);
        }

        /// <summary>
        /// Gets a stream by ID.
        /// </summary>
        public static DataStream GetStream(ByteString streamId)
        {
            var key = GetStreamKey(streamId);
            var stored = Storage.Get(Storage.CurrentContext, key);
            if (stored == null) return null;
            return (DataStream)StdLib.Deserialize(stored);
        }

        /// <summary>
        /// Deactivates a stream. Only owner can call.
        /// </summary>
        public static void DeactivateStream(ByteString streamId)
        {
            var stream = GetStream(streamId);
            if (stream == null) throw new Exception("Stream not found");
            if (!Runtime.CheckWitness(stream.Owner)) throw new Exception("Only owner");

            stream.IsActive = false;
            StoreStream(streamId, stream);
        }

        /// <summary>
        /// Gets the total stream count.
        /// </summary>
        public static BigInteger GetStreamCount()
        {
            var stored = Storage.Get(Storage.CurrentContext, new byte[] { PREFIX_STREAM_COUNT });
            return stored != null ? (BigInteger)stored : 0;
        }

        // ==================== Subscription Management ====================

        /// <summary>
        /// Subscribes to a stream. Called via Gateway.
        /// </summary>
        public static void Subscribe(ByteString subscriptionId, ByteString streamId, UInt160 subscriber)
        {
            RequireGateway();
            RequireNotPaused();

            // Validate stream exists and is active
            var stream = GetStream(streamId);
            if (stream == null) throw new Exception("Stream not found");
            if (!stream.IsActive) throw new Exception("Stream not active");

            var subscription = new StreamSubscription
            {
                SubscriptionId = subscriptionId,
                StreamId = streamId,
                Subscriber = subscriber,
                CreatedAt = Runtime.Time,
                IsActive = true
            };
            StoreSubscription(subscriptionId, subscription);

            OnStreamSubscribed(subscriptionId, streamId, subscriber);
        }

        /// <summary>
        /// Gets a subscription by ID.
        /// </summary>
        public static StreamSubscription GetSubscription(ByteString subscriptionId)
        {
            var key = GetSubscriptionKey(subscriptionId);
            var stored = Storage.Get(Storage.CurrentContext, key);
            if (stored == null) return null;
            return (StreamSubscription)StdLib.Deserialize(stored);
        }

        /// <summary>
        /// Unsubscribes from a stream. Only subscriber can call.
        /// </summary>
        public static void Unsubscribe(ByteString subscriptionId)
        {
            var subscription = GetSubscription(subscriptionId);
            if (subscription == null) throw new Exception("Subscription not found");
            if (!Runtime.CheckWitness(subscription.Subscriber)) throw new Exception("Only subscriber");

            subscription.IsActive = false;
            StoreSubscription(subscriptionId, subscription);
        }

        // ==================== Data Publishing ====================

        /// <summary>
        /// Processes a stream data publish request. Called via Gateway.
        /// </summary>
        public static void ProcessRequest(ByteString requestId, UInt160 requester, ByteString payload)
        {
            RequireGateway();
            RequireNotPaused();

            var requestData = (StreamRequestData)StdLib.Deserialize(payload);

            // Validate stream exists and is active
            var stream = GetStream(requestData.StreamId);
            if (stream == null) throw new Exception("Stream not found");
            if (!stream.IsActive) throw new Exception("Stream not active");

            var dataHash = CryptoLib.Sha256(requestData.Data);

            // Store report
            var report = new StreamReport
            {
                ReportId = requestId,
                StreamId = requestData.StreamId,
                Publisher = requester,
                DataHash = dataHash,
                Timestamp = Runtime.Time,
                Status = 0
            };
            StoreReport(requestId, report);
        }

        /// <summary>
        /// Delivers stream data. Called via Gateway from Service Layer.
        /// </summary>
        public static void DeliverResponse(ByteString requestId, bool success, ByteString proof, ByteString signature)
        {
            RequireGateway();

            var report = GetReport(requestId);
            if (report == null) throw new Exception("Report not found");
            if (report.Status != 0) throw new Exception("Already processed");

            report.Status = success ? (byte)1 : (byte)2;
            StoreReport(requestId, report);

            if (success)
            {
                // Update stream last update time
                var stream = GetStream(report.StreamId);
                if (stream != null)
                {
                    stream.LastUpdateAt = Runtime.Time;
                    StoreStream(report.StreamId, stream);
                }

                OnStreamDataPublished(report.StreamId, report.Timestamp, report.DataHash, proof);
            }
        }

        /// <summary>
        /// Gets a report by ID.
        /// </summary>
        public static StreamReport GetReport(ByteString reportId)
        {
            var key = GetReportKey(reportId);
            var stored = Storage.Get(Storage.CurrentContext, key);
            if (stored == null) return null;
            return (StreamReport)StdLib.Deserialize(stored);
        }

        // ==================== Helper Methods ====================

        private static void RequireAdmin()
        {
            if (!Runtime.CheckWitness(GetAdmin()))
                throw new Exception("Only admin");
        }

        private static void RequireGateway()
        {
            var gateway = GetGateway();
            if (gateway == UInt160.Zero) throw new Exception("Gateway not configured");
            if (Runtime.CallingScriptHash != gateway) throw new Exception("Only gateway");
        }

        private static void RequireNotPaused()
        {
            if (IsPaused()) throw new Exception("Contract paused");
        }

        private static byte[] GetStreamKey(ByteString streamId)
        {
            return Helper.Concat(new byte[] { PREFIX_STREAM }, streamId);
        }

        private static byte[] GetSubscriptionKey(ByteString subscriptionId)
        {
            return Helper.Concat(new byte[] { PREFIX_SUBSCRIPTION }, subscriptionId);
        }

        private static byte[] GetReportKey(ByteString reportId)
        {
            return Helper.Concat(new byte[] { PREFIX_REPORT }, reportId);
        }

        private static void StoreStream(ByteString streamId, DataStream stream)
        {
            var key = GetStreamKey(streamId);
            Storage.Put(Storage.CurrentContext, key, StdLib.Serialize(stream));
        }

        private static void StoreSubscription(ByteString subscriptionId, StreamSubscription subscription)
        {
            var key = GetSubscriptionKey(subscriptionId);
            Storage.Put(Storage.CurrentContext, key, StdLib.Serialize(subscription));
        }

        private static void StoreReport(ByteString reportId, StreamReport report)
        {
            var key = GetReportKey(reportId);
            Storage.Put(Storage.CurrentContext, key, StdLib.Serialize(report));
        }

        private static void IncrementStreamCount()
        {
            var count = GetStreamCount();
            Storage.Put(Storage.CurrentContext, new byte[] { PREFIX_STREAM_COUNT }, count + 1);
        }

        public static void Update(ByteString nefFile, string manifest)
        {
            RequireAdmin();
            ContractManagement.Update(nefFile, manifest, null);
        }
    }

    public class StreamRequestData
    {
        public ByteString StreamId;
        public ByteString Data;
    }

    public class DataStream
    {
        public ByteString StreamId;
        public UInt160 Owner;
        public string StreamType;
        public BigInteger UpdateInterval;
        public ByteString Config;
        public BigInteger CreatedAt;
        public bool IsActive;
        public BigInteger LastUpdateAt;
    }

    public class StreamSubscription
    {
        public ByteString SubscriptionId;
        public ByteString StreamId;
        public UInt160 Subscriber;
        public BigInteger CreatedAt;
        public bool IsActive;
    }

    public class StreamReport
    {
        public ByteString ReportId;
        public ByteString StreamId;
        public UInt160 Publisher;
        public ByteString DataHash;
        public BigInteger Timestamp;
        public byte Status;
    }
}
