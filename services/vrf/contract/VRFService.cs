using Neo;
using Neo.SmartContract;
using Neo.SmartContract.Framework;
using Neo.SmartContract.Framework.Attributes;
using Neo.SmartContract.Framework.Native;
using Neo.SmartContract.Framework.Services;
using ServiceLayer.Common;
using System;
using System.ComponentModel;
using System.Numerics;

namespace ServiceLayer.VRF
{
    /// <summary>
    /// VRFService - On-chain logic for VRF (Verifiable Random Function) requests.
    ///
    /// This contract ONLY interacts with ServiceLayerGateway:
    /// - Receives requests via onRequest() from Gateway
    /// - Emits events for TEE to monitor
    /// - Receives fulfillment via onFulfill() from Gateway
    /// - Stores VRF proofs for on-chain verification
    ///
    /// Flow:
    /// 1. UserContract → Gateway.RequestService("vrf", payload) → VRFService.onRequest()
    /// 2. VRFService emits VRFRequest event
    /// 3. TEE monitors event, generates VRF, signs result
    /// 4. TEE → Gateway.FulfillRequest() → VRFService.onFulfill()
    /// 5. Gateway → UserContract.callback()
    /// </summary>
    [DisplayName("VRFService")]
    [ManifestExtra("Author", "R3E Network")]
    [ManifestExtra("Description", "VRF Service Contract - Verifiable Random Numbers")]
    [ManifestExtra("Version", "2.0.0")]
    [ContractPermission("*", "*")]
    public partial class VRFService : ServiceContractBase
    {
        // ============================================================================
        // Events - Monitored by TEE
        // ============================================================================

        /// <summary>
        /// Emitted when a new VRF request is created.
        /// TEE monitors this event to generate randomness.
        /// </summary>
        [DisplayName("VRFRequest")]
        public static event Action<BigInteger, UInt160, byte[], BigInteger> OnVRFRequest;
        // requestId, userContract, seed, numWords

        /// <summary>Emitted when VRF is fulfilled with proof</summary>
        [DisplayName("VRFFulfilled")]
        public static event Action<BigInteger, byte[], byte[]> OnVRFFulfilled;
        // requestId, randomWords, proof

        // ============================================================================
        // Contract Lifecycle
        // ============================================================================

        [DisplayName("_deploy")]
        public static void Deploy(object data, bool update)
        {
            if (update) return;
        }
    }
}
