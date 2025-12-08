using Neo;
using Neo.SmartContract;
using Neo.SmartContract.Framework;
using Neo.SmartContract.Framework.Services;
using System;

namespace ServiceLayer.Common
{
    /// <summary>
    /// ServiceContractBase - Base class for all Service Layer service contracts.
    ///
    /// Provides common functionality:
    /// - Gateway management (set/get/require)
    /// - Standard storage prefix for gateway
    ///
    /// All service contracts should inherit from this base class to ensure
    /// consistent gateway interaction patterns.
    /// </summary>
    public abstract class ServiceContractBase : SmartContract
    {
        // Storage prefix for gateway address
        protected const byte PREFIX_GATEWAY = 0x01;

        // ============================================================================
        // Gateway Management
        // ============================================================================

        /// <summary>
        /// Sets the gateway contract address.
        /// Can only be called by the current gateway (if set) or anyone (if not set).
        /// </summary>
        public static void SetGateway(UInt160 gateway)
        {
            UInt160 currentGateway = GetGateway();
            if (currentGateway != null)
            {
                if (Runtime.CallingScriptHash != currentGateway)
                    throw new Exception("Only gateway can update");
            }
            Storage.Put(Storage.CurrentContext, new byte[] { PREFIX_GATEWAY }, gateway);
        }

        /// <summary>
        /// Gets the current gateway contract address.
        /// </summary>
        public static UInt160 GetGateway()
        {
            return (UInt160)Storage.Get(Storage.CurrentContext, new byte[] { PREFIX_GATEWAY });
        }

        /// <summary>
        /// Validates that the caller is the registered gateway.
        /// Throws if gateway is not set or caller is not the gateway.
        /// </summary>
        protected static void RequireGateway()
        {
            UInt160 gateway = GetGateway();
            if (gateway == null) throw new Exception("Gateway not set");
            if (Runtime.CallingScriptHash != gateway) throw new Exception("Only gateway");
        }
    }
}
