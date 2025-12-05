using Neo;
using Neo.SmartContract;
using Neo.SmartContract.Framework;
using Neo.SmartContract.Framework.Attributes;
using Neo.SmartContract.Framework.Native;
using Neo.SmartContract.Framework.Services;
using System;
using System.ComponentModel;
using System.Numerics;

namespace ServiceLayer.Common
{
    /// <summary>
    /// Base class for all Service Layer contracts.
    /// Provides common functionality and access control.
    /// </summary>
    public abstract class ServiceLayerBase : SmartContract
    {
        // Storage prefixes
        protected const byte PREFIX_ADMIN = 0x01;
        protected const byte PREFIX_GATEWAY = 0x02;
        protected const byte PREFIX_SERVICE_LAYER = 0x03;
        protected const byte PREFIX_PAUSED = 0x04;

        /// <summary>
        /// Contract owner/admin address.
        /// </summary>
        [InitialValue("NZ8zBpRh9zLaQ5DdUz4qUb9bf5MZvELyq6", ContractParameterType.Hash160)]
        protected static readonly UInt160 InitialAdmin = default;

        // ==================== Admin Management ====================

        /// <summary>
        /// Gets the current admin address.
        /// </summary>
        public static UInt160 GetAdmin()
        {
            var stored = Storage.Get(Storage.CurrentContext, new byte[] { PREFIX_ADMIN });
            return stored != null ? (UInt160)stored : InitialAdmin;
        }

        /// <summary>
        /// Sets a new admin address. Only callable by current admin.
        /// </summary>
        public static void SetAdmin(UInt160 newAdmin)
        {
            RequireAdmin();
            if (!newAdmin.IsValid) throw new Exception("Invalid admin address");
            Storage.Put(Storage.CurrentContext, new byte[] { PREFIX_ADMIN }, newAdmin);
            OnAdminChanged(GetAdmin(), newAdmin);
        }

        // ==================== Gateway Management ====================

        /// <summary>
        /// Gets the gateway contract address.
        /// </summary>
        public static UInt160 GetGateway()
        {
            var stored = Storage.Get(Storage.CurrentContext, new byte[] { PREFIX_GATEWAY });
            return stored != null ? (UInt160)stored : UInt160.Zero;
        }

        /// <summary>
        /// Sets the gateway contract address. Only callable by admin.
        /// </summary>
        public static void SetGateway(UInt160 gateway)
        {
            RequireAdmin();
            if (!gateway.IsValid) throw new Exception("Invalid gateway address");
            Storage.Put(Storage.CurrentContext, new byte[] { PREFIX_GATEWAY }, gateway);
            OnGatewayChanged(gateway);
        }

        // ==================== Service Layer Management ====================

        /// <summary>
        /// Gets the service layer (TEE) address.
        /// </summary>
        public static UInt160 GetServiceLayer()
        {
            var stored = Storage.Get(Storage.CurrentContext, new byte[] { PREFIX_SERVICE_LAYER });
            return stored != null ? (UInt160)stored : UInt160.Zero;
        }

        /// <summary>
        /// Sets the service layer address. Only callable by admin.
        /// </summary>
        public static void SetServiceLayer(UInt160 serviceLayer)
        {
            RequireAdmin();
            if (!serviceLayer.IsValid) throw new Exception("Invalid service layer address");
            Storage.Put(Storage.CurrentContext, new byte[] { PREFIX_SERVICE_LAYER }, serviceLayer);
            OnServiceLayerChanged(serviceLayer);
        }

        // ==================== Pause Management ====================

        /// <summary>
        /// Checks if the contract is paused.
        /// </summary>
        public static bool IsPaused()
        {
            var stored = Storage.Get(Storage.CurrentContext, new byte[] { PREFIX_PAUSED });
            return stored != null && (BigInteger)stored == 1;
        }

        /// <summary>
        /// Pauses the contract. Only callable by admin.
        /// </summary>
        public static void Pause()
        {
            RequireAdmin();
            Storage.Put(Storage.CurrentContext, new byte[] { PREFIX_PAUSED }, 1);
            OnPaused();
        }

        /// <summary>
        /// Unpauses the contract. Only callable by admin.
        /// </summary>
        public static void Unpause()
        {
            RequireAdmin();
            Storage.Delete(Storage.CurrentContext, new byte[] { PREFIX_PAUSED });
            OnUnpaused();
        }

        // ==================== Access Control Helpers ====================

        /// <summary>
        /// Requires the caller to be the admin.
        /// </summary>
        protected static void RequireAdmin()
        {
            if (!Runtime.CheckWitness(GetAdmin()))
                throw new Exception("Only admin can call this method");
        }

        /// <summary>
        /// Requires the caller to be the gateway contract.
        /// </summary>
        protected static void RequireGateway()
        {
            var gateway = GetGateway();
            if (gateway == UInt160.Zero)
                throw new Exception("Gateway not configured");
            if (Runtime.CallingScriptHash != gateway)
                throw new Exception("Only gateway can call this method");
        }

        /// <summary>
        /// Requires the caller to be the service layer.
        /// </summary>
        protected static void RequireServiceLayer()
        {
            var serviceLayer = GetServiceLayer();
            if (serviceLayer == UInt160.Zero)
                throw new Exception("Service layer not configured");
            if (!Runtime.CheckWitness(serviceLayer))
                throw new Exception("Only service layer can call this method");
        }

        /// <summary>
        /// Requires the contract to not be paused.
        /// </summary>
        protected static void RequireNotPaused()
        {
            if (IsPaused())
                throw new Exception("Contract is paused");
        }

        // ==================== Events ====================

        [DisplayName("AdminChanged")]
        public static event Action<UInt160, UInt160> OnAdminChanged;

        [DisplayName("GatewayChanged")]
        public static event Action<UInt160> OnGatewayChanged;

        [DisplayName("ServiceLayerChanged")]
        public static event Action<UInt160> OnServiceLayerChanged;

        [DisplayName("Paused")]
        public static event Action OnPaused;

        [DisplayName("Unpaused")]
        public static event Action OnUnpaused;

        // ==================== Update/Destroy ====================

        /// <summary>
        /// Updates the contract. Only callable by admin.
        /// </summary>
        public static void Update(ByteString nefFile, string manifest)
        {
            RequireAdmin();
            ContractManagement.Update(nefFile, manifest, null);
        }

        /// <summary>
        /// Destroys the contract. Only callable by admin.
        /// </summary>
        public static void Destroy()
        {
            RequireAdmin();
            ContractManagement.Destroy();
        }
    }

    /// <summary>
    /// Request status enumeration.
    /// </summary>
    public enum RequestStatus : byte
    {
        Pending = 0,
        Processed = 1,
        Failed = 2,
        Cancelled = 3
    }

    /// <summary>
    /// Service request structure stored in contracts.
    /// </summary>
    public class ServiceRequest
    {
        public ByteString RequestId;
        public UInt160 Requester;
        public UInt160 CallbackContract;
        public string CallbackMethod;
        public ByteString Payload;
        public BigInteger GasDeposit;
        public RequestStatus Status;
        public BigInteger CreatedAt;
        public BigInteger ProcessedAt;
    }
}
