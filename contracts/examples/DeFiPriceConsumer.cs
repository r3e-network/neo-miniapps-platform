using Neo;
using Neo.SmartContract;
using Neo.SmartContract.Framework;
using Neo.SmartContract.Framework.Attributes;
using Neo.SmartContract.Framework.Native;
using Neo.SmartContract.Framework.Services;
using System;
using System.ComponentModel;
using System.Numerics;

namespace ServiceLayer.Examples
{
    /// <summary>
    /// DeFiPriceConsumer - A DeFi contract using Service Layer DataFeeds and Oracle.
    ///
    /// Features:
    /// - Read on-chain price feeds (DataFeeds pattern)
    /// - Request custom price data via Oracle
    /// - Collateral valuation for lending/borrowing
    /// - Liquidation triggers based on price thresholds
    ///
    /// Demonstrates both:
    /// - Pattern 2: Push (DataFeeds) - read latest on-chain prices
    /// - Pattern 1: Request-Response (Oracle) - fetch custom external data
    /// </summary>
    [DisplayName("DeFiPriceConsumer")]
    [ManifestExtra("Author", "R3E Network")]
    [ManifestExtra("Description", "DeFi Price Consumer using Service Layer DataFeeds & Oracle")]
    [ManifestExtra("Version", "1.0.0")]
    [ContractPermission("*", "*")]
    public class DeFiPriceConsumer : SmartContract
    {
        private const byte PREFIX_OWNER = 0x01;
        private const byte PREFIX_GATEWAY = 0x02;
        private const byte PREFIX_DATAFEEDS = 0x03;
        private const byte PREFIX_POSITION = 0x10;
        private const byte PREFIX_ORACLE_REQUEST = 0x20;
        private const byte PREFIX_CUSTOM_PRICE = 0x30;

        // Collateral ratio: 150% (1.5x in basis points = 15000)
        private const int MIN_COLLATERAL_RATIO = 15000;
        private const int BASIS_POINTS = 10000;

        // Price decimals (8 decimals for prices)
        private const int PRICE_DECIMALS = 100000000;

        // Events
        [DisplayName("PositionOpened")]
        public static event Action<BigInteger, UInt160, BigInteger, BigInteger> OnPositionOpened;

        [DisplayName("PositionClosed")]
        public static event Action<BigInteger, UInt160> OnPositionClosed;

        [DisplayName("PositionLiquidated")]
        public static event Action<BigInteger, UInt160, BigInteger> OnPositionLiquidated;

        [DisplayName("PriceRequested")]
        public static event Action<BigInteger, string> OnPriceRequested;

        [DisplayName("PriceReceived")]
        public static event Action<string, BigInteger, ulong> OnPriceReceived;

        // ============================================================================
        // Contract Lifecycle
        // ============================================================================

        [DisplayName("_deploy")]
        public static void Deploy(object data, bool update)
        {
            if (update) return;
            Transaction tx = (Transaction)Runtime.ScriptContainer;
            Storage.Put(Storage.CurrentContext, new byte[] { PREFIX_OWNER }, tx.Sender);
        }

        // ============================================================================
        // Configuration
        // ============================================================================

        public static void SetGateway(UInt160 gateway)
        {
            RequireOwner();
            Storage.Put(Storage.CurrentContext, new byte[] { PREFIX_GATEWAY }, gateway);
        }

        public static void SetDataFeedsContract(UInt160 dataFeeds)
        {
            RequireOwner();
            Storage.Put(Storage.CurrentContext, new byte[] { PREFIX_DATAFEEDS }, dataFeeds);
        }

        public static UInt160 GetGateway() =>
            (UInt160)Storage.Get(Storage.CurrentContext, new byte[] { PREFIX_GATEWAY });

        public static UInt160 GetDataFeedsContract() =>
            (UInt160)Storage.Get(Storage.CurrentContext, new byte[] { PREFIX_DATAFEEDS });

        private static UInt160 GetOwner() =>
            (UInt160)Storage.Get(Storage.CurrentContext, new byte[] { PREFIX_OWNER });

        private static void RequireOwner()
        {
            if (!Runtime.CheckWitness(GetOwner())) throw new Exception("Owner only");
        }

        // ============================================================================
        // DataFeeds Integration (Pattern 2: Push - Read Latest Prices)
        // ============================================================================

        /// <summary>
        /// Get latest price from DataFeeds contract.
        /// DataFeeds contract is automatically updated by TEE.
        /// </summary>
        public static PriceFeed GetLatestPrice(string pair)
        {
            UInt160 dataFeeds = GetDataFeedsContract();
            if (dataFeeds == null) throw new Exception("DataFeeds not configured");

            // Call DataFeeds contract to get latest price
            object[] result = (object[])Contract.Call(dataFeeds, "getLatestPrice", CallFlags.ReadOnly,
                new object[] { pair });

            if (result == null || result.Length < 3) return null;

            return new PriceFeed
            {
                Pair = pair,
                Price = (BigInteger)result[0],
                Timestamp = (ulong)result[1],
                Decimals = (int)result[2]
            };
        }

        /// <summary>Check if price is fresh (within maxAge seconds)</summary>
        public static bool IsPriceFresh(string pair, ulong maxAgeMs)
        {
            PriceFeed feed = GetLatestPrice(pair);
            if (feed == null) return false;
            return Runtime.Time <= feed.Timestamp + maxAgeMs;
        }

        // ============================================================================
        // Oracle Integration (Pattern 1: Request-Response)
        // ============================================================================

        /// <summary>
        /// Request custom price data via Oracle.
        /// Use this for prices not available in DataFeeds.
        /// </summary>
        public static BigInteger RequestCustomPrice(string pair, string apiUrl, string jsonPath)
        {
            UInt160 gateway = GetGateway();
            if (gateway == null) throw new Exception("Gateway not configured");

            OraclePayload payload = new OraclePayload
            {
                Url = apiUrl,
                Method = "GET",
                JsonPath = jsonPath
            };

            byte[] payloadBytes = (byte[])StdLib.Serialize(payload);

            BigInteger requestId = (BigInteger)Contract.Call(gateway, "requestService", CallFlags.All,
                new object[] { "oracle", payloadBytes, "onOracleCallback" });

            // Store request mapping
            StorageMap requestMap = new StorageMap(Storage.CurrentContext, PREFIX_ORACLE_REQUEST);
            requestMap.Put(requestId.ToByteArray(), pair);

            OnPriceRequested(requestId, pair);

            return requestId;
        }

        /// <summary>Oracle callback handler</summary>
        public static void OnOracleCallback(BigInteger requestId, bool success, byte[] result, string error)
        {
            UInt160 gateway = GetGateway();
            if (Runtime.CallingScriptHash != gateway)
                throw new Exception("Only gateway can callback");

            StorageMap requestMap = new StorageMap(Storage.CurrentContext, PREFIX_ORACLE_REQUEST);
            ByteString pair = requestMap.Get(requestId.ToByteArray());
            if (pair == null) throw new Exception("Unknown request");

            if (!success) return;

            // Parse and store custom price
            BigInteger price = new BigInteger(result);

            StorageMap priceMap = new StorageMap(Storage.CurrentContext, PREFIX_CUSTOM_PRICE);
            CustomPrice customPrice = new CustomPrice
            {
                Price = price,
                Timestamp = Runtime.Time
            };
            priceMap.Put(pair, StdLib.Serialize(customPrice));

            OnPriceReceived(pair, price, Runtime.Time);

            // Clean up
            requestMap.Delete(requestId.ToByteArray());
        }

        /// <summary>Get custom price from Oracle</summary>
        public static CustomPrice GetCustomPrice(string pair)
        {
            StorageMap priceMap = new StorageMap(Storage.CurrentContext, PREFIX_CUSTOM_PRICE);
            ByteString data = priceMap.Get(pair);
            if (data == null) return null;
            return (CustomPrice)StdLib.Deserialize(data);
        }

        // ============================================================================
        // DeFi Position Management (Using Price Feeds)
        // ============================================================================

        /// <summary>Open a collateralized position</summary>
        public static void OnNEP17Payment(UInt160 from, BigInteger amount, object data)
        {
            if (Runtime.CallingScriptHash != GAS.Hash) throw new Exception("Only GAS accepted");
            if (amount <= 0) throw new Exception("Invalid amount");

            // Get current GAS price
            PriceFeed gasPrice = GetLatestPrice("GAS/USD");
            if (gasPrice == null) throw new Exception("GAS price not available");

            // Calculate collateral value in USD
            BigInteger collateralValueUSD = amount * gasPrice.Price / PRICE_DECIMALS;

            // Create position
            BigInteger positionId = GetNextPositionId();
            Position position = new Position
            {
                Id = positionId,
                Owner = from,
                Collateral = amount,
                CollateralValueUSD = collateralValueUSD,
                OpenPrice = gasPrice.Price,
                OpenTime = Runtime.Time,
                IsOpen = true
            };

            SavePosition(positionId, position);

            OnPositionOpened(positionId, from, amount, collateralValueUSD);
        }

        /// <summary>Close a position and withdraw collateral</summary>
        public static void ClosePosition(BigInteger positionId)
        {
            Transaction tx = (Transaction)Runtime.ScriptContainer;

            Position position = GetPosition(positionId);
            if (position == null) throw new Exception("Position not found");
            if (!position.IsOpen) throw new Exception("Position already closed");
            if (position.Owner != tx.Sender) throw new Exception("Not position owner");

            // Return collateral
            GAS.Transfer(Runtime.ExecutingScriptHash, position.Owner, position.Collateral, null);

            position.IsOpen = false;
            position.CloseTime = Runtime.Time;
            SavePosition(positionId, position);

            OnPositionClosed(positionId, position.Owner);
        }

        /// <summary>Check if a position is liquidatable</summary>
        public static bool IsLiquidatable(BigInteger positionId)
        {
            Position position = GetPosition(positionId);
            if (position == null || !position.IsOpen) return false;

            PriceFeed gasPrice = GetLatestPrice("GAS/USD");
            if (gasPrice == null) return false;

            // Calculate current collateral ratio
            BigInteger currentValueUSD = position.Collateral * gasPrice.Price / PRICE_DECIMALS;
            BigInteger ratio = currentValueUSD * BASIS_POINTS / position.CollateralValueUSD;

            return ratio < MIN_COLLATERAL_RATIO;
        }

        /// <summary>Liquidate an undercollateralized position</summary>
        public static void Liquidate(BigInteger positionId)
        {
            if (!IsLiquidatable(positionId))
                throw new Exception("Position not liquidatable");

            Position position = GetPosition(positionId);
            Transaction tx = (Transaction)Runtime.ScriptContainer;

            // Reward liquidator (5% of collateral)
            BigInteger reward = position.Collateral * 5 / 100;
            GAS.Transfer(Runtime.ExecutingScriptHash, tx.Sender, reward, null);

            // Return remainder to position owner
            BigInteger remainder = position.Collateral - reward;
            if (remainder > 0)
            {
                GAS.Transfer(Runtime.ExecutingScriptHash, position.Owner, remainder, null);
            }

            position.IsOpen = false;
            position.CloseTime = Runtime.Time;
            position.LiquidatedBy = tx.Sender;
            SavePosition(positionId, position);

            OnPositionLiquidated(positionId, tx.Sender, reward);
        }

        // ============================================================================
        // Query Functions
        // ============================================================================

        public static Position GetPosition(BigInteger positionId)
        {
            StorageMap positionMap = new StorageMap(Storage.CurrentContext, PREFIX_POSITION);
            ByteString data = positionMap.Get(positionId.ToByteArray());
            if (data == null) return null;
            return (Position)StdLib.Deserialize(data);
        }

        private static void SavePosition(BigInteger positionId, Position position)
        {
            StorageMap positionMap = new StorageMap(Storage.CurrentContext, PREFIX_POSITION);
            positionMap.Put(positionId.ToByteArray(), StdLib.Serialize(position));
        }

        private static BigInteger GetNextPositionId()
        {
            byte[] key = new byte[] { 0xFE };
            BigInteger id = (BigInteger)Storage.Get(Storage.CurrentContext, key);
            id += 1;
            Storage.Put(Storage.CurrentContext, key, id);
            return id;
        }

        public static int GetMinCollateralRatio() => MIN_COLLATERAL_RATIO;
    }

    // ============================================================================
    // Data Structures
    // ============================================================================

    public class PriceFeed
    {
        public string Pair;
        public BigInteger Price;
        public ulong Timestamp;
        public int Decimals;
    }

    public class CustomPrice
    {
        public BigInteger Price;
        public ulong Timestamp;
    }

    public class Position
    {
        public BigInteger Id;
        public UInt160 Owner;
        public BigInteger Collateral;
        public BigInteger CollateralValueUSD;
        public BigInteger OpenPrice;
        public ulong OpenTime;
        public ulong CloseTime;
        public bool IsOpen;
        public UInt160 LiquidatedBy;
    }

    public class OraclePayload
    {
        public string Url;
        public string Method;
        public string Headers;
        public string Body;
        public string JsonPath;
    }
}
