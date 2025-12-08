using Neo;
using Neo.SmartContract.Framework;
using Neo.SmartContract.Framework.Attributes;
using Neo.SmartContract.Framework.Services;
using ServiceLayer.Common;
using System;
using System.ComponentModel;
using System.Numerics;

namespace ServiceLayer.Oracle
{
    /// <summary>
    /// OracleService - On-chain logic for external data oracle requests.
    ///
    /// This contract ONLY interacts with ServiceLayerGateway:
    /// - Receives requests via OnRequest() from Gateway
    /// - Emits events for TEE to monitor
    /// - Receives fulfillment via OnFulfill() from Gateway
    ///
    /// Flow:
    /// 1. UserContract -> Gateway.RequestService("oracle", payload) -> OracleService.OnRequest()
    /// 2. OracleService emits OracleRequest event
    /// 3. TEE monitors event, fetches external data, signs result
    /// 4. TEE -> Gateway.FulfillRequest() -> OracleService.OnFulfill()
    /// 5. Gateway -> UserContract.callback()
    /// </summary>
    [DisplayName("OracleService")]
    [ManifestExtra("Author", "R3E Network")]
    [ManifestExtra("Description", "Oracle Service Contract - External Data Feeds")]
    [ManifestExtra("Version", "2.0.0")]
    [ContractPermission("*", "*")]
    public partial class OracleService : ServiceContractBase
    {
        // Storage prefixes (PREFIX_GATEWAY inherited from base)
        private const byte PREFIX_REQUEST = 0x10;
        private const byte PREFIX_RESULT = 0x20;

        // ============================================================================
        // Events - Monitored by TEE
        // ============================================================================

        /// <summary>
        /// Emitted when a new Oracle request is created.
        /// TEE monitors this event to fetch external data.
        /// </summary>
        [DisplayName("OracleRequest")]
        public static event Action<BigInteger, UInt160, string, string, string, string> OnOracleRequest;
        // requestId, userContract, url, method, headers, jsonPath

        /// <summary>Emitted when Oracle request is fulfilled</summary>
        [DisplayName("OracleFulfilled")]
        public static event Action<BigInteger, byte[]> OnOracleFulfilled;
        // requestId, result

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
