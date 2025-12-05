using Neo;
using Neo.SmartContract;
using Neo.SmartContract.Framework;
using Neo.SmartContract.Framework.Attributes;
using Neo.SmartContract.Framework.Native;
using Neo.SmartContract.Framework.Services;
using System;
using System.ComponentModel;
using System.Numerics;

namespace ServiceLayer.DTA
{
    /// <summary>
    /// DTAService - Data Trust Authority contract.
    ///
    /// Provides data provenance, attestation, and trust verification services.
    ///
    /// Flow:
    /// 1. User submits data for attestation via Gateway
    /// 2. Service Layer (TEE) verifies data integrity and provenance
    /// 3. Attestation certificate issued on-chain
    ///
    /// Supports multi-party attestation and trust scoring.
    /// </summary>
    [DisplayName("DTAService")]
    [ManifestExtra("Author", "R3E Network")]
    [ManifestExtra("Description", "DTA Service - Data Trust Authority")]
    [ContractPermission("*", "*")]
    public class DTAService : SmartContract
    {
        // ==================== Storage Prefixes ====================
        private const byte PREFIX_ADMIN = 0x01;
        private const byte PREFIX_GATEWAY = 0x02;
        private const byte PREFIX_ATTESTATION = 0x10;
        private const byte PREFIX_TRUST_ANCHOR = 0x20;
        private const byte PREFIX_ATTESTATION_COUNT = 0x30;
        private const byte PREFIX_PAUSED = 0x40;

        [InitialValue("NZ8zBpRh9zLaQ5DdUz4qUb9bf5MZvELyq6", ContractParameterType.Hash160)]
        private static readonly UInt160 InitialAdmin = default;

        // ==================== Events ====================

        /// <summary>
        /// Emitted when an attestation request is created.
        /// </summary>
        [DisplayName("AttestationRequest")]
        public static event Action<ByteString, UInt160, ByteString, string> OnAttestationRequest;
        // Parameters: requestId, requester, dataHash, attestationType

        /// <summary>
        /// Emitted when attestation is issued.
        /// </summary>
        [DisplayName("AttestationIssued")]
        public static event Action<ByteString, ByteString, BigInteger, ByteString> OnAttestationIssued;
        // Parameters: attestationId, dataHash, trustScore, certificate

        /// <summary>
        /// Emitted when a trust anchor is registered.
        /// </summary>
        [DisplayName("TrustAnchorRegistered")]
        public static event Action<ByteString, UInt160, string> OnTrustAnchorRegistered;
        // Parameters: anchorId, authority, anchorType

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

        // ==================== Trust Anchor Management ====================

        /// <summary>
        /// Registers a trust anchor. Only admin can call.
        /// </summary>
        public static void RegisterTrustAnchor(ByteString anchorId, UInt160 authority, string anchorType, ByteString publicKey)
        {
            RequireAdmin();

            // Validate anchor type
            if (anchorType != "root" && anchorType != "intermediate" && anchorType != "leaf")
                throw new Exception("Invalid anchor type");

            // Check if anchor already exists
            if (GetTrustAnchor(anchorId) != null)
                throw new Exception("Anchor already exists");

            var anchor = new TrustAnchor
            {
                AnchorId = anchorId,
                Authority = authority,
                AnchorType = anchorType,
                PublicKey = publicKey,
                RegisteredAt = Runtime.Time,
                IsActive = true
            };
            StoreTrustAnchor(anchorId, anchor);

            OnTrustAnchorRegistered(anchorId, authority, anchorType);
        }

        /// <summary>
        /// Gets a trust anchor by ID.
        /// </summary>
        public static TrustAnchor GetTrustAnchor(ByteString anchorId)
        {
            var key = GetTrustAnchorKey(anchorId);
            var stored = Storage.Get(Storage.CurrentContext, key);
            if (stored == null) return null;
            return (TrustAnchor)StdLib.Deserialize(stored);
        }

        /// <summary>
        /// Revokes a trust anchor. Only admin can call.
        /// </summary>
        public static void RevokeTrustAnchor(ByteString anchorId)
        {
            RequireAdmin();
            var anchor = GetTrustAnchor(anchorId);
            if (anchor == null) throw new Exception("Anchor not found");

            anchor.IsActive = false;
            StoreTrustAnchor(anchorId, anchor);
        }

        // ==================== Attestation Operations ====================

        /// <summary>
        /// Processes an attestation request. Called via Gateway.
        /// </summary>
        public static void ProcessRequest(ByteString requestId, UInt160 requester, ByteString payload)
        {
            RequireGateway();
            RequireNotPaused();

            var requestData = (DTARequestData)StdLib.Deserialize(payload);

            // Validate attestation type
            if (requestData.AttestationType != "integrity" &&
                requestData.AttestationType != "provenance" &&
                requestData.AttestationType != "authenticity")
                throw new Exception("Invalid attestation type");

            var dataHash = CryptoLib.Sha256(requestData.Data);

            // Store attestation request
            var attestation = new Attestation
            {
                AttestationId = requestId,
                Requester = requester,
                DataHash = dataHash,
                AttestationType = requestData.AttestationType,
                TrustAnchorId = requestData.TrustAnchorId,
                Status = 0,
                CreatedAt = Runtime.Time
            };
            StoreAttestation(requestId, attestation);

            OnAttestationRequest(requestId, requester, dataHash, requestData.AttestationType);
        }

        /// <summary>
        /// Delivers attestation result. Called via Gateway from Service Layer.
        /// </summary>
        public static void DeliverResponse(ByteString requestId, bool success, ByteString attestationData, ByteString signature)
        {
            RequireGateway();

            var attestation = GetAttestation(requestId);
            if (attestation == null) throw new Exception("Attestation not found");
            if (attestation.Status != 0) throw new Exception("Already processed");

            attestation.Status = success ? (byte)1 : (byte)2;
            attestation.ProcessedAt = Runtime.Time;

            BigInteger trustScore = 0;
            ByteString certificate = null;

            if (success && attestationData != null)
            {
                var result = (AttestationResult)StdLib.Deserialize(attestationData);
                trustScore = result.TrustScore;
                certificate = result.Certificate;
                attestation.TrustScore = trustScore;
                attestation.Certificate = certificate;
            }

            StoreAttestation(requestId, attestation);

            // Increment attestation count
            IncrementAttestationCount();

            if (success)
            {
                OnAttestationIssued(requestId, attestation.DataHash, trustScore, certificate);
            }
        }

        /// <summary>
        /// Gets an attestation by ID.
        /// </summary>
        public static Attestation GetAttestation(ByteString attestationId)
        {
            var key = GetAttestationKey(attestationId);
            var stored = Storage.Get(Storage.CurrentContext, key);
            if (stored == null) return null;
            return (Attestation)StdLib.Deserialize(stored);
        }

        /// <summary>
        /// Verifies an attestation certificate.
        /// </summary>
        public static bool VerifyAttestation(ByteString attestationId, ByteString certificate)
        {
            var attestation = GetAttestation(attestationId);
            if (attestation == null) return false;
            if (attestation.Status != 1) return false;

            // Verify certificate matches
            return attestation.Certificate == certificate;
        }

        /// <summary>
        /// Gets the total attestation count.
        /// </summary>
        public static BigInteger GetAttestationCount()
        {
            var stored = Storage.Get(Storage.CurrentContext, new byte[] { PREFIX_ATTESTATION_COUNT });
            return stored != null ? (BigInteger)stored : 0;
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

        private static byte[] GetAttestationKey(ByteString attestationId)
        {
            return Helper.Concat(new byte[] { PREFIX_ATTESTATION }, attestationId);
        }

        private static byte[] GetTrustAnchorKey(ByteString anchorId)
        {
            return Helper.Concat(new byte[] { PREFIX_TRUST_ANCHOR }, anchorId);
        }

        private static void StoreAttestation(ByteString attestationId, Attestation attestation)
        {
            var key = GetAttestationKey(attestationId);
            Storage.Put(Storage.CurrentContext, key, StdLib.Serialize(attestation));
        }

        private static void StoreTrustAnchor(ByteString anchorId, TrustAnchor anchor)
        {
            var key = GetTrustAnchorKey(anchorId);
            Storage.Put(Storage.CurrentContext, key, StdLib.Serialize(anchor));
        }

        private static void IncrementAttestationCount()
        {
            var count = GetAttestationCount();
            Storage.Put(Storage.CurrentContext, new byte[] { PREFIX_ATTESTATION_COUNT }, count + 1);
        }

        public static void Update(ByteString nefFile, string manifest)
        {
            RequireAdmin();
            ContractManagement.Update(nefFile, manifest, null);
        }
    }

    public class DTARequestData
    {
        public ByteString Data;
        public string AttestationType;
        public ByteString TrustAnchorId;
    }

    public class Attestation
    {
        public ByteString AttestationId;
        public UInt160 Requester;
        public ByteString DataHash;
        public string AttestationType;
        public ByteString TrustAnchorId;
        public byte Status;
        public BigInteger CreatedAt;
        public BigInteger ProcessedAt;
        public BigInteger TrustScore;
        public ByteString Certificate;
    }

    public class AttestationResult
    {
        public BigInteger TrustScore;
        public ByteString Certificate;
    }

    public class TrustAnchor
    {
        public ByteString AnchorId;
        public UInt160 Authority;
        public string AnchorType;
        public ByteString PublicKey;
        public BigInteger RegisteredAt;
        public bool IsActive;
    }
}
