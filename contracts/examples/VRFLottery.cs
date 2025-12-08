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
    /// VRFLottery - A provably fair lottery using Service Layer VRF.
    ///
    /// Features:
    /// - Players buy tickets for each round
    /// - Winner selected using verifiable randomness from TEE
    /// - Transparent and auditable random selection
    ///
    /// Flow:
    /// 1. Admin starts a new round
    /// 2. Players buy tickets (NEP-17 payment)
    /// 3. Admin closes ticket sales and requests random number
    /// 4. VRF callback selects winner
    /// 5. Winner can claim prize
    /// </summary>
    [DisplayName("VRFLottery")]
    [ManifestExtra("Author", "R3E Network")]
    [ManifestExtra("Description", "Provably Fair Lottery using Service Layer VRF")]
    [ManifestExtra("Version", "1.0.0")]
    [ContractPermission("*", "*")]
    public class VRFLottery : SmartContract
    {
        private const byte PREFIX_OWNER = 0x01;
        private const byte PREFIX_GATEWAY = 0x02;
        private const byte PREFIX_ROUND = 0x10;
        private const byte PREFIX_TICKET = 0x20;
        private const byte PREFIX_VRF_REQUEST = 0x30;
        private const byte PREFIX_CURRENT_ROUND = 0x40;

        private const long TICKET_PRICE = 100000000; // 1 GAS

        // Events
        [DisplayName("RoundStarted")]
        public static event Action<BigInteger, ulong> OnRoundStarted;

        [DisplayName("TicketPurchased")]
        public static event Action<BigInteger, UInt160, BigInteger> OnTicketPurchased;

        [DisplayName("RoundClosed")]
        public static event Action<BigInteger, BigInteger> OnRoundClosed;

        [DisplayName("WinnerSelected")]
        public static event Action<BigInteger, UInt160, BigInteger> OnWinnerSelected;

        [DisplayName("PrizeClaimed")]
        public static event Action<BigInteger, UInt160, BigInteger> OnPrizeClaimed;

        // ============================================================================
        // Contract Lifecycle
        // ============================================================================

        [DisplayName("_deploy")]
        public static void Deploy(object data, bool update)
        {
            if (update) return;
            Transaction tx = (Transaction)Runtime.ScriptContainer;
            Storage.Put(Storage.CurrentContext, new byte[] { PREFIX_OWNER }, tx.Sender);
            Storage.Put(Storage.CurrentContext, new byte[] { PREFIX_CURRENT_ROUND }, 0);
        }

        // ============================================================================
        // Configuration
        // ============================================================================

        public static void SetGateway(UInt160 gateway)
        {
            RequireOwner();
            Storage.Put(Storage.CurrentContext, new byte[] { PREFIX_GATEWAY }, gateway);
        }

        public static UInt160 GetGateway() =>
            (UInt160)Storage.Get(Storage.CurrentContext, new byte[] { PREFIX_GATEWAY });

        private static UInt160 GetOwner() =>
            (UInt160)Storage.Get(Storage.CurrentContext, new byte[] { PREFIX_OWNER });

        private static void RequireOwner()
        {
            if (!Runtime.CheckWitness(GetOwner())) throw new Exception("Owner only");
        }

        // ============================================================================
        // Round Management
        // ============================================================================

        /// <summary>Start a new lottery round</summary>
        public static BigInteger StartRound()
        {
            RequireOwner();

            BigInteger roundId = GetCurrentRound() + 1;
            Storage.Put(Storage.CurrentContext, new byte[] { PREFIX_CURRENT_ROUND }, roundId);

            RoundInfo round = new RoundInfo
            {
                RoundId = roundId,
                Status = RoundStatus.Open,
                StartTime = Runtime.Time,
                TicketCount = 0,
                PrizePool = 0
            };

            SaveRound(roundId, round);
            OnRoundStarted(roundId, Runtime.Time);

            return roundId;
        }

        /// <summary>Close ticket sales and request random number</summary>
        public static BigInteger CloseRound(BigInteger roundId)
        {
            RequireOwner();

            RoundInfo round = GetRound(roundId);
            if (round == null) throw new Exception("Round not found");
            if (round.Status != RoundStatus.Open) throw new Exception("Round not open");
            if (round.TicketCount == 0) throw new Exception("No tickets sold");

            UInt160 gateway = GetGateway();
            if (gateway == null) throw new Exception("Gateway not set");

            // Build VRF request payload
            VRFPayload payload = new VRFPayload
            {
                Seed = Helper.Concat(roundId.ToByteArray(), ((BigInteger)Runtime.Time).ToByteArray()),
                NumWords = 1
            };

            byte[] payloadBytes = (byte[])StdLib.Serialize(payload);

            // Request VRF from gateway
            BigInteger requestId = (BigInteger)Contract.Call(gateway, "requestService", CallFlags.All,
                new object[] { "vrf", payloadBytes, "onVRFCallback" });

            // Store mapping: requestId -> roundId
            StorageMap vrfMap = new StorageMap(Storage.CurrentContext, PREFIX_VRF_REQUEST);
            vrfMap.Put(requestId.ToByteArray(), roundId.ToByteArray());

            // Update round status
            round.Status = RoundStatus.Drawing;
            round.VRFRequestId = requestId;
            SaveRound(roundId, round);

            OnRoundClosed(roundId, requestId);

            return requestId;
        }

        // ============================================================================
        // Ticket Purchase
        // ============================================================================

        /// <summary>Called when GAS is received for ticket purchase</summary>
        public static void OnNEP17Payment(UInt160 from, BigInteger amount, object data)
        {
            if (Runtime.CallingScriptHash != GAS.Hash) throw new Exception("Only GAS accepted");
            if (amount < TICKET_PRICE) throw new Exception("Insufficient payment");

            BigInteger roundId = GetCurrentRound();
            RoundInfo round = GetRound(roundId);
            if (round == null || round.Status != RoundStatus.Open)
                throw new Exception("No open round");

            BigInteger ticketCount = amount / TICKET_PRICE;
            BigInteger ticketValue = ticketCount * TICKET_PRICE;

            // Refund excess
            if (amount > ticketValue)
            {
                GAS.Transfer(Runtime.ExecutingScriptHash, from, amount - ticketValue, null);
            }

            // Register tickets
            for (BigInteger i = 0; i < ticketCount; i++)
            {
                BigInteger ticketId = round.TicketCount + i;
                StorageMap ticketMap = new StorageMap(Storage.CurrentContext, PREFIX_TICKET);
                byte[] key = Helper.Concat(roundId.ToByteArray(), ticketId.ToByteArray());
                ticketMap.Put(key, from);
            }

            // Update round
            round.TicketCount += ticketCount;
            round.PrizePool += ticketValue;
            SaveRound(roundId, round);

            OnTicketPurchased(roundId, from, ticketCount);
        }

        // ============================================================================
        // VRF Callback
        // ============================================================================

        /// <summary>Callback from Service Layer VRF</summary>
        public static void OnVRFCallback(BigInteger requestId, bool success, byte[] result, string error)
        {
            UInt160 gateway = GetGateway();
            if (Runtime.CallingScriptHash != gateway)
                throw new Exception("Only gateway can callback");

            // Get round for this VRF request
            StorageMap vrfMap = new StorageMap(Storage.CurrentContext, PREFIX_VRF_REQUEST);
            ByteString roundIdBytes = vrfMap.Get(requestId.ToByteArray());
            if (roundIdBytes == null) throw new Exception("Unknown VRF request");

            BigInteger roundId = new BigInteger((byte[])roundIdBytes);
            RoundInfo round = GetRound(roundId);

            if (!success)
            {
                // VRF failed - reopen round for retry
                round.Status = RoundStatus.Open;
                SaveRound(roundId, round);
                return;
            }

            // Select winner using VRF result
            BigInteger randomNumber = new BigInteger(result);
            BigInteger winningTicket = randomNumber % round.TicketCount;
            if (winningTicket < 0) winningTicket += round.TicketCount;

            // Get winner address
            StorageMap ticketMap = new StorageMap(Storage.CurrentContext, PREFIX_TICKET);
            byte[] key = Helper.Concat(roundId.ToByteArray(), winningTicket.ToByteArray());
            UInt160 winner = (UInt160)ticketMap.Get(key);

            // Update round with winner
            round.Status = RoundStatus.Completed;
            round.Winner = winner;
            round.WinningTicket = winningTicket;
            round.VRFResult = result;
            SaveRound(roundId, round);

            OnWinnerSelected(roundId, winner, round.PrizePool);

            // Clean up VRF mapping
            vrfMap.Delete(requestId.ToByteArray());
        }

        // ============================================================================
        // Prize Claim
        // ============================================================================

        /// <summary>Winner claims their prize</summary>
        public static void ClaimPrize(BigInteger roundId)
        {
            RoundInfo round = GetRound(roundId);
            if (round == null) throw new Exception("Round not found");
            if (round.Status != RoundStatus.Completed) throw new Exception("Round not completed");
            if (round.PrizeClaimed) throw new Exception("Prize already claimed");

            Transaction tx = (Transaction)Runtime.ScriptContainer;
            if (tx.Sender != round.Winner) throw new Exception("Not the winner");

            // Transfer prize (90% to winner, 10% fee)
            BigInteger prize = round.PrizePool * 9 / 10;
            GAS.Transfer(Runtime.ExecutingScriptHash, round.Winner, prize, null);

            round.PrizeClaimed = true;
            SaveRound(roundId, round);

            OnPrizeClaimed(roundId, round.Winner, prize);
        }

        // ============================================================================
        // Query Functions
        // ============================================================================

        public static BigInteger GetCurrentRound() =>
            (BigInteger)Storage.Get(Storage.CurrentContext, new byte[] { PREFIX_CURRENT_ROUND });

        public static RoundInfo GetRound(BigInteger roundId)
        {
            StorageMap roundMap = new StorageMap(Storage.CurrentContext, PREFIX_ROUND);
            ByteString data = roundMap.Get(roundId.ToByteArray());
            if (data == null) return null;
            return (RoundInfo)StdLib.Deserialize(data);
        }

        private static void SaveRound(BigInteger roundId, RoundInfo round)
        {
            StorageMap roundMap = new StorageMap(Storage.CurrentContext, PREFIX_ROUND);
            roundMap.Put(roundId.ToByteArray(), StdLib.Serialize(round));
        }

        public static BigInteger GetTicketPrice() => TICKET_PRICE;

        public static UInt160 GetTicketOwner(BigInteger roundId, BigInteger ticketId)
        {
            StorageMap ticketMap = new StorageMap(Storage.CurrentContext, PREFIX_TICKET);
            byte[] key = Helper.Concat(roundId.ToByteArray(), ticketId.ToByteArray());
            return (UInt160)ticketMap.Get(key);
        }
    }

    public enum RoundStatus : byte
    {
        Open = 0,
        Drawing = 1,
        Completed = 2,
        Cancelled = 3
    }

    public class RoundInfo
    {
        public BigInteger RoundId;
        public RoundStatus Status;
        public ulong StartTime;
        public BigInteger TicketCount;
        public BigInteger PrizePool;
        public BigInteger VRFRequestId;
        public byte[] VRFResult;
        public UInt160 Winner;
        public BigInteger WinningTicket;
        public bool PrizeClaimed;
    }

    public class VRFPayload
    {
        public byte[] Seed;
        public BigInteger NumWords;
    }
}
