using Neo;
using Neo.SmartContract;
using Neo.SmartContract.Framework;
using Neo.SmartContract.Framework.Attributes;
using Neo.SmartContract.Framework.Native;
using Neo.SmartContract.Framework.Services;
using System;
using System.ComponentModel;
using System.Numerics;

namespace ServiceLayer.DataLink
{
    /// <summary>
    /// DataLinkService - Data synchronization contract.
    ///
    /// Provides secure data synchronization between on-chain and off-chain systems.
    ///
    /// Flow:
    /// 1. User configures data sync job via Gateway
    /// 2. Service Layer (TEE) monitors source and syncs to target
    /// 3. Sync status and confirmations delivered on-chain
    ///
    /// Supports bidirectional sync with conflict resolution.
    /// </summary>
    [DisplayName("DataLinkService")]
    [ManifestExtra("Author", "R3E Network")]
    [ManifestExtra("Description", "DataLink Service - Secure data synchronization")]
    [ContractPermission("*", "*")]
    public class DataLinkService : SmartContract
    {
        // ==================== Storage Prefixes ====================
        private const byte PREFIX_ADMIN = 0x01;
        private const byte PREFIX_GATEWAY = 0x02;
        private const byte PREFIX_SYNC_JOB = 0x10;
        private const byte PREFIX_SYNC_RECORD = 0x11;
        private const byte PREFIX_JOB_COUNT = 0x20;
        private const byte PREFIX_PAUSED = 0x30;

        [InitialValue("NZ8zBpRh9zLaQ5DdUz4qUb9bf5MZvELyq6", ContractParameterType.Hash160)]
        private static readonly UInt160 InitialAdmin = default;

        // ==================== Events ====================

        /// <summary>
        /// Emitted when a sync job is created.
        /// </summary>
        [DisplayName("SyncJobCreated")]
        public static event Action<ByteString, UInt160, string, ByteString, ByteString> OnSyncJobCreated;
        // Parameters: jobId, owner, syncType, sourceEndpoint, targetEndpoint

        /// <summary>
        /// Emitted when data sync is requested.
        /// </summary>
        [DisplayName("SyncRequest")]
        public static event Action<ByteString, ByteString, ByteString> OnSyncRequest;
        // Parameters: syncId, jobId, dataHash

        /// <summary>
        /// Emitted when sync completes.
        /// </summary>
        [DisplayName("SyncComplete")]
        public static event Action<ByteString, bool, BigInteger, ByteString> OnSyncComplete;
        // Parameters: syncId, success, recordCount, proof

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

        // ==================== Sync Job Management ====================

        /// <summary>
        /// Creates a sync job. Called via Gateway.
        /// </summary>
        public static void CreateSyncJob(ByteString jobId, UInt160 owner, string syncType, ByteString sourceEndpoint, ByteString targetEndpoint, ByteString config)
        {
            RequireGateway();
            RequireNotPaused();

            // Validate sync type
            if (syncType != "push" && syncType != "pull" && syncType != "bidirectional")
                throw new Exception("Invalid sync type");

            // Check if job already exists
            if (GetSyncJob(jobId) != null)
                throw new Exception("Job already exists");

            var job = new SyncJob
            {
                JobId = jobId,
                Owner = owner,
                SyncType = syncType,
                SourceEndpoint = sourceEndpoint,
                TargetEndpoint = targetEndpoint,
                Config = config,
                CreatedAt = Runtime.Time,
                IsActive = true,
                LastSyncAt = 0
            };
            StoreSyncJob(jobId, job);

            IncrementJobCount();

            OnSyncJobCreated(jobId, owner, syncType, sourceEndpoint, targetEndpoint);
        }

        /// <summary>
        /// Gets a sync job by ID.
        /// </summary>
        public static SyncJob GetSyncJob(ByteString jobId)
        {
            var key = GetSyncJobKey(jobId);
            var stored = Storage.Get(Storage.CurrentContext, key);
            if (stored == null) return null;
            return (SyncJob)StdLib.Deserialize(stored);
        }

        /// <summary>
        /// Deactivates a sync job. Only owner can call.
        /// </summary>
        public static void DeactivateSyncJob(ByteString jobId)
        {
            var job = GetSyncJob(jobId);
            if (job == null) throw new Exception("Job not found");
            if (!Runtime.CheckWitness(job.Owner)) throw new Exception("Only owner");

            job.IsActive = false;
            StoreSyncJob(jobId, job);
        }

        /// <summary>
        /// Gets the total job count.
        /// </summary>
        public static BigInteger GetJobCount()
        {
            var stored = Storage.Get(Storage.CurrentContext, new byte[] { PREFIX_JOB_COUNT });
            return stored != null ? (BigInteger)stored : 0;
        }

        // ==================== Data Sync Operations ====================

        /// <summary>
        /// Processes a data sync request. Called via Gateway.
        /// </summary>
        public static void ProcessRequest(ByteString requestId, UInt160 requester, ByteString payload)
        {
            RequireGateway();
            RequireNotPaused();

            var requestData = (DataLinkRequestData)StdLib.Deserialize(payload);

            // Validate job exists and is active
            var job = GetSyncJob(requestData.JobId);
            if (job == null) throw new Exception("Job not found");
            if (!job.IsActive) throw new Exception("Job not active");

            var dataHash = CryptoLib.Sha256(requestData.Data);

            // Store sync record
            var record = new SyncRecord
            {
                SyncId = requestId,
                JobId = requestData.JobId,
                Requester = requester,
                DataHash = dataHash,
                Status = 0,
                CreatedAt = Runtime.Time
            };
            StoreSyncRecord(requestId, record);

            OnSyncRequest(requestId, requestData.JobId, dataHash);
        }

        /// <summary>
        /// Delivers sync result. Called via Gateway from Service Layer.
        /// </summary>
        public static void DeliverResponse(ByteString requestId, bool success, ByteString syncData, ByteString signature)
        {
            RequireGateway();

            var record = GetSyncRecord(requestId);
            if (record == null) throw new Exception("Record not found");
            if (record.Status != 0) throw new Exception("Already processed");

            record.Status = success ? (byte)1 : (byte)2;
            record.ProcessedAt = Runtime.Time;
            StoreSyncRecord(requestId, record);

            // Update job last sync time
            var job = GetSyncJob(record.JobId);
            if (job != null)
            {
                job.LastSyncAt = Runtime.Time;
                StoreSyncJob(record.JobId, job);
            }

            BigInteger recordCount = 0;
            ByteString proof = null;

            if (success && syncData != null)
            {
                var result = (SyncResult)StdLib.Deserialize(syncData);
                recordCount = result.RecordCount;
                proof = result.Proof;
            }

            OnSyncComplete(requestId, success, recordCount, proof);
        }

        /// <summary>
        /// Gets a sync record by ID.
        /// </summary>
        public static SyncRecord GetSyncRecord(ByteString syncId)
        {
            var key = GetSyncRecordKey(syncId);
            var stored = Storage.Get(Storage.CurrentContext, key);
            if (stored == null) return null;
            return (SyncRecord)StdLib.Deserialize(stored);
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

        private static byte[] GetSyncJobKey(ByteString jobId)
        {
            return Helper.Concat(new byte[] { PREFIX_SYNC_JOB }, jobId);
        }

        private static byte[] GetSyncRecordKey(ByteString syncId)
        {
            return Helper.Concat(new byte[] { PREFIX_SYNC_RECORD }, syncId);
        }

        private static void StoreSyncJob(ByteString jobId, SyncJob job)
        {
            var key = GetSyncJobKey(jobId);
            Storage.Put(Storage.CurrentContext, key, StdLib.Serialize(job));
        }

        private static void StoreSyncRecord(ByteString syncId, SyncRecord record)
        {
            var key = GetSyncRecordKey(syncId);
            Storage.Put(Storage.CurrentContext, key, StdLib.Serialize(record));
        }

        private static void IncrementJobCount()
        {
            var count = GetJobCount();
            Storage.Put(Storage.CurrentContext, new byte[] { PREFIX_JOB_COUNT }, count + 1);
        }

        public static void Update(ByteString nefFile, string manifest)
        {
            RequireAdmin();
            ContractManagement.Update(nefFile, manifest, null);
        }
    }

    public class DataLinkRequestData
    {
        public ByteString JobId;
        public ByteString Data;
    }

    public class SyncJob
    {
        public ByteString JobId;
        public UInt160 Owner;
        public string SyncType;
        public ByteString SourceEndpoint;
        public ByteString TargetEndpoint;
        public ByteString Config;
        public BigInteger CreatedAt;
        public bool IsActive;
        public BigInteger LastSyncAt;
    }

    public class SyncRecord
    {
        public ByteString SyncId;
        public ByteString JobId;
        public UInt160 Requester;
        public ByteString DataHash;
        public byte Status;
        public BigInteger CreatedAt;
        public BigInteger ProcessedAt;
    }

    public class SyncResult
    {
        public BigInteger RecordCount;
        public ByteString Proof;
    }
}
