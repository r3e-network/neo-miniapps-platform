using Neo;
using Neo.SmartContract;
using Neo.SmartContract.Framework;
using Neo.SmartContract.Framework.Attributes;
using Neo.SmartContract.Framework.Native;
using Neo.SmartContract.Framework.Services;
using System;
using System.Numerics;

namespace ServiceLayer.DApps
{
    /// <summary>
    /// MegaLottery - Decentralized lottery powered by Service Layer VRF and Automation.
    ///
    /// Features:
    /// - Fair winner selection using VRF (Verifiable Random Function)
    /// - Automated draws using Automation service
    /// - Multiple prize tiers
    /// - Transparent and verifiable
    ///
    /// Flow:
    /// 1. Admin creates a lottery round
    /// 2. Users buy tickets by sending GAS
    /// 3. Automation triggers draw at scheduled time
    /// 4. VRF generates random numbers for winner selection
    /// 5. Winners can claim prizes
    /// </summary>
    [DisplayName("MegaLottery")]
    [ManifestExtra("Author", "R3E Network")]
    [ManifestExtra("Description", "Decentralized Lottery DApp powered by Service Layer")]
    [ContractPermission("*", "*")]
    public class MegaLottery : SmartContract
    {
        // ==================== Storage Prefixes ====================
        private const byte PREFIX_ADMIN = 0x01;
        private const byte PREFIX_GATEWAY = 0x02;
        private const byte PREFIX_ROUND = 0x10;
        private const byte PREFIX_TICKET = 0x20;
        private const byte PREFIX_USER_TICKETS = 0x30;
        private const byte PREFIX_WINNER = 0x40;
        private const byte PREFIX_CURRENT_ROUND = 0x50;
        private const byte PREFIX_AUTOMATION_TRIGGER = 0x60;
        private const byte PREFIX_VRF_REQUEST = 0x70;
        private const byte PREFIX_CONFIG = 0x80;

        // Round status
        private const byte STATUS_OPEN = 0;
        private const byte STATUS_DRAWING = 1;
        private const byte STATUS_COMPLETED = 2;
        private const byte STATUS_CANCELLED = 3;

        [InitialValue("NZHf1NJvz1tvELGLWZjhpb3NqZJFFqMSbR", ContractParameterType.Hash160)]
        private static readonly UInt160 InitialAdmin = default;

        // ==================== Events ====================

        [DisplayName("RoundCreated")]
        public static event Action<BigInteger, BigInteger, BigInteger, BigInteger> OnRoundCreated;
        // roundId, ticketPrice, endTime, jackpot

        [DisplayName("TicketPurchased")]
        public static event Action<BigInteger, UInt160, BigInteger, BigInteger[]> OnTicketPurchased;
        // roundId, buyer, ticketId, numbers

        [DisplayName("DrawStarted")]
        public static event Action<BigInteger, ByteString> OnDrawStarted;
        // roundId, vrfRequestId

        [DisplayName("WinnerSelected")]
        public static event Action<BigInteger, UInt160, BigInteger, BigInteger> OnWinnerSelected;
        // roundId, winner, prize, tier

        [DisplayName("PrizeClaimed")]
        public static event Action<BigInteger, UInt160, BigInteger> OnPrizeClaimed;
        // roundId, winner, amount

        [DisplayName("RoundCompleted")]
        public static event Action<BigInteger, BigInteger[]> OnRoundCompleted;
        // roundId, winningNumbers

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

        // ==================== Configuration ====================

        public static void SetConfig(BigInteger ticketPrice, BigInteger drawInterval, BigInteger minTickets)
        {
            RequireAdmin();
            var config = new LotteryConfig
            {
                TicketPrice = ticketPrice,
                DrawInterval = drawInterval,
                MinTickets = minTickets,
                PrizeTiers = new BigInteger[] { 50, 30, 20 } // 50%, 30%, 20% for tiers
            };
            Storage.Put(Storage.CurrentContext, new byte[] { PREFIX_CONFIG }, StdLib.Serialize(config));
        }

        public static LotteryConfig GetConfig()
        {
            var stored = Storage.Get(Storage.CurrentContext, new byte[] { PREFIX_CONFIG });
            if (stored == null)
            {
                return new LotteryConfig
                {
                    TicketPrice = 1_00000000, // 1 GAS
                    DrawInterval = 86400000,   // 24 hours in ms
                    MinTickets = 10,
                    PrizeTiers = new BigInteger[] { 50, 30, 20 }
                };
            }
            return (LotteryConfig)StdLib.Deserialize(stored);
        }

        // ==================== Round Management ====================

        /// <summary>
        /// Creates a new lottery round.
        /// </summary>
        public static BigInteger CreateRound(BigInteger initialJackpot)
        {
            RequireAdmin();

            var config = GetConfig();
            var currentRound = GetCurrentRoundId();
            var newRoundId = currentRound + 1;

            var round = new LotteryRound
            {
                RoundId = newRoundId,
                TicketPrice = config.TicketPrice,
                StartTime = Runtime.Time,
                EndTime = Runtime.Time + (ulong)config.DrawInterval,
                Jackpot = initialJackpot,
                TicketCount = 0,
                Status = STATUS_OPEN,
                WinningNumbers = null,
                VRFRequestId = null,
                AutomationTriggerId = null
            };

            StoreRound(newRoundId, round);
            SetCurrentRoundId(newRoundId);

            // Register automation trigger for draw
            RegisterDrawTrigger(newRoundId, round.EndTime);

            OnRoundCreated(newRoundId, config.TicketPrice, round.EndTime, initialJackpot);

            return newRoundId;
        }

        /// <summary>
        /// Buys lottery tickets.
        /// </summary>
        public static BigInteger BuyTicket(BigInteger roundId, BigInteger[] numbers)
        {
            var round = GetRound(roundId);
            if (round == null) throw new Exception("Round not found");
            if (round.Status != STATUS_OPEN) throw new Exception("Round not open");
            if (Runtime.Time >= round.EndTime) throw new Exception("Round ended");

            // Validate numbers (6 numbers between 1-49)
            if (numbers.Length != 6) throw new Exception("Must select 6 numbers");
            for (int i = 0; i < numbers.Length; i++)
            {
                if (numbers[i] < 1 || numbers[i] > 49) throw new Exception("Numbers must be 1-49");
            }

            var buyer = Runtime.CallingScriptHash;
            var ticketId = round.TicketCount + 1;

            // Create ticket
            var ticket = new LotteryTicket
            {
                TicketId = ticketId,
                RoundId = roundId,
                Buyer = buyer,
                Numbers = numbers,
                PurchaseTime = Runtime.Time,
                Claimed = false
            };

            StoreTicket(roundId, ticketId, ticket);
            AddUserTicket(buyer, roundId, ticketId);

            // Update round
            round.TicketCount = ticketId;
            round.Jackpot += round.TicketPrice;
            StoreRound(roundId, round);

            OnTicketPurchased(roundId, buyer, ticketId, numbers);

            return ticketId;
        }

        /// <summary>
        /// Called by Automation service to trigger the draw.
        /// </summary>
        public static void TriggerDraw(ByteString triggerId, ByteString args)
        {
            RequireGateway();

            var roundId = GetCurrentRoundId();
            var round = GetRound(roundId);
            if (round == null) throw new Exception("Round not found");
            if (round.Status != STATUS_OPEN) throw new Exception("Round not open");

            var config = GetConfig();
            if (round.TicketCount < config.MinTickets)
            {
                // Not enough tickets, extend round
                round.EndTime = Runtime.Time + (ulong)config.DrawInterval;
                StoreRound(roundId, round);
                return;
            }

            // Update status
            round.Status = STATUS_DRAWING;
            StoreRound(roundId, round);

            // Request VRF for random numbers
            RequestVRFForDraw(roundId);

            OnDrawStarted(roundId, round.VRFRequestId);
        }

        /// <summary>
        /// VRF callback with random numbers.
        /// </summary>
        public static void OnVRFCallback(ByteString requestId, ByteString result)
        {
            RequireGateway();

            var roundId = GetRoundByVRFRequest(requestId);
            if (roundId == 0) throw new Exception("Round not found for VRF request");

            var round = GetRound(roundId);
            if (round.Status != STATUS_DRAWING) throw new Exception("Round not in drawing state");

            // Parse random numbers from VRF result
            var randomData = (VRFResult)StdLib.Deserialize(result);
            var winningNumbers = GenerateWinningNumbers(randomData.Randomness);

            // Store winning numbers
            round.WinningNumbers = winningNumbers;
            round.Status = STATUS_COMPLETED;
            StoreRound(roundId, round);

            // Calculate and distribute prizes
            DistributePrizes(roundId, winningNumbers);

            OnRoundCompleted(roundId, winningNumbers);

            // Create next round
            CreateRound(0);
        }

        /// <summary>
        /// Claims prize for a winning ticket.
        /// </summary>
        public static void ClaimPrize(BigInteger roundId, BigInteger ticketId)
        {
            var ticket = GetTicket(roundId, ticketId);
            if (ticket == null) throw new Exception("Ticket not found");
            if (ticket.Claimed) throw new Exception("Already claimed");

            var caller = Runtime.CallingScriptHash;
            if (ticket.Buyer != caller) throw new Exception("Not ticket owner");

            var round = GetRound(roundId);
            if (round.Status != STATUS_COMPLETED) throw new Exception("Round not completed");

            // Calculate prize
            var matchCount = CountMatches(ticket.Numbers, round.WinningNumbers);
            var prize = CalculatePrize(round.Jackpot, matchCount);

            if (prize == 0) throw new Exception("No prize");

            // Mark as claimed
            ticket.Claimed = true;
            StoreTicket(roundId, ticketId, ticket);

            // Transfer prize
            GAS.Transfer(Runtime.ExecutingScriptHash, ticket.Buyer, prize, null);

            OnPrizeClaimed(roundId, ticket.Buyer, prize);
        }

        // ==================== Query Methods ====================

        public static BigInteger GetCurrentRoundId()
        {
            var stored = Storage.Get(Storage.CurrentContext, new byte[] { PREFIX_CURRENT_ROUND });
            return stored != null ? (BigInteger)stored : 0;
        }

        public static LotteryRound GetRound(BigInteger roundId)
        {
            var key = GetRoundKey(roundId);
            var stored = Storage.Get(Storage.CurrentContext, key);
            if (stored == null) return null;
            return (LotteryRound)StdLib.Deserialize(stored);
        }

        public static LotteryTicket GetTicket(BigInteger roundId, BigInteger ticketId)
        {
            var key = GetTicketKey(roundId, ticketId);
            var stored = Storage.Get(Storage.CurrentContext, key);
            if (stored == null) return null;
            return (LotteryTicket)StdLib.Deserialize(stored);
        }

        public static BigInteger GetJackpot(BigInteger roundId)
        {
            var round = GetRound(roundId);
            return round != null ? round.Jackpot : 0;
        }

        // ==================== NEP-17 Payment Handler ====================

        public static void OnNEP17Payment(UInt160 from, BigInteger amount, object data)
        {
            if (Runtime.CallingScriptHash != GAS.Hash) return;

            // Auto-buy ticket if data contains numbers
            if (data != null)
            {
                var numbers = (BigInteger[])data;
                var roundId = GetCurrentRoundId();
                var config = GetConfig();

                if (amount >= config.TicketPrice)
                {
                    // Buy ticket
                    var ticket = new LotteryTicket
                    {
                        RoundId = roundId,
                        Buyer = from,
                        Numbers = numbers,
                        PurchaseTime = Runtime.Time,
                        Claimed = false
                    };

                    var round = GetRound(roundId);
                    var ticketId = round.TicketCount + 1;
                    ticket.TicketId = ticketId;

                    StoreTicket(roundId, ticketId, ticket);
                    AddUserTicket(from, roundId, ticketId);

                    round.TicketCount = ticketId;
                    round.Jackpot += config.TicketPrice;
                    StoreRound(roundId, round);

                    OnTicketPurchased(roundId, from, ticketId, numbers);

                    // Refund excess
                    var excess = amount - config.TicketPrice;
                    if (excess > 0)
                    {
                        GAS.Transfer(Runtime.ExecutingScriptHash, from, excess, null);
                    }
                }
            }
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

        private static void SetCurrentRoundId(BigInteger roundId)
        {
            Storage.Put(Storage.CurrentContext, new byte[] { PREFIX_CURRENT_ROUND }, roundId);
        }

        private static byte[] GetRoundKey(BigInteger roundId)
        {
            return Helper.Concat(new byte[] { PREFIX_ROUND }, (ByteString)roundId);
        }

        private static byte[] GetTicketKey(BigInteger roundId, BigInteger ticketId)
        {
            var key = Helper.Concat(new byte[] { PREFIX_TICKET }, (ByteString)roundId);
            return Helper.Concat(key, (ByteString)ticketId);
        }

        private static void StoreRound(BigInteger roundId, LotteryRound round)
        {
            var key = GetRoundKey(roundId);
            Storage.Put(Storage.CurrentContext, key, StdLib.Serialize(round));
        }

        private static void StoreTicket(BigInteger roundId, BigInteger ticketId, LotteryTicket ticket)
        {
            var key = GetTicketKey(roundId, ticketId);
            Storage.Put(Storage.CurrentContext, key, StdLib.Serialize(ticket));
        }

        private static void AddUserTicket(UInt160 user, BigInteger roundId, BigInteger ticketId)
        {
            var key = Helper.Concat(new byte[] { PREFIX_USER_TICKETS }, (ByteString)user);
            key = Helper.Concat(key, (ByteString)roundId);
            key = Helper.Concat(key, (ByteString)ticketId);
            Storage.Put(Storage.CurrentContext, key, 1);
        }

        private static void RegisterDrawTrigger(BigInteger roundId, BigInteger endTime)
        {
            var gateway = GetGateway();
            if (gateway == UInt160.Zero) return;

            // Create automation trigger for draw
            var payload = StdLib.Serialize(new AutomationPayload
            {
                TriggerType = 3, // ONCE
                Target = Runtime.ExecutingScriptHash,
                Method = "TriggerDraw",
                Schedule = (ByteString)endTime,
                GasLimit = 10_00000000,
                MaxExecutions = 1
            });

            Contract.Call(gateway, "request", CallFlags.All, new object[]
            {
                "automation",
                Runtime.ExecutingScriptHash,
                "OnAutomationCallback",
                payload,
                1_00000000
            });
        }

        private static void RequestVRFForDraw(BigInteger roundId)
        {
            var gateway = GetGateway();
            if (gateway == UInt160.Zero) throw new Exception("Gateway not configured");

            // Create VRF request
            var seed = Helper.Concat((ByteString)roundId, (ByteString)Runtime.Time);
            var payload = StdLib.Serialize(new VRFPayload
            {
                Seed = seed,
                NumWords = 6
            });

            var requestId = (ByteString)Contract.Call(gateway, "request", CallFlags.All, new object[]
            {
                "vrf",
                Runtime.ExecutingScriptHash,
                "OnVRFCallback",
                payload,
                3_00000000
            });

            // Store mapping
            var key = Helper.Concat(new byte[] { PREFIX_VRF_REQUEST }, requestId);
            Storage.Put(Storage.CurrentContext, key, roundId);

            var round = GetRound(roundId);
            round.VRFRequestId = requestId;
            StoreRound(roundId, round);
        }

        private static BigInteger GetRoundByVRFRequest(ByteString requestId)
        {
            var key = Helper.Concat(new byte[] { PREFIX_VRF_REQUEST }, requestId);
            var stored = Storage.Get(Storage.CurrentContext, key);
            return stored != null ? (BigInteger)stored : 0;
        }

        private static BigInteger[] GenerateWinningNumbers(ByteString randomness)
        {
            var numbers = new BigInteger[6];
            var used = new Map<BigInteger, bool>();

            int index = 0;
            int offset = 0;

            while (index < 6 && offset < randomness.Length - 1)
            {
                var num = ((BigInteger)randomness[offset] % 49) + 1;
                if (!used.HasKey(num))
                {
                    numbers[index] = num;
                    used[num] = true;
                    index++;
                }
                offset++;
            }

            return numbers;
        }

        private static BigInteger CountMatches(BigInteger[] ticketNumbers, BigInteger[] winningNumbers)
        {
            BigInteger matches = 0;
            for (int i = 0; i < ticketNumbers.Length; i++)
            {
                for (int j = 0; j < winningNumbers.Length; j++)
                {
                    if (ticketNumbers[i] == winningNumbers[j])
                    {
                        matches++;
                        break;
                    }
                }
            }
            return matches;
        }

        private static BigInteger CalculatePrize(BigInteger jackpot, BigInteger matches)
        {
            if (matches == 6) return jackpot * 50 / 100;  // 50% for 6 matches
            if (matches == 5) return jackpot * 20 / 100;  // 20% for 5 matches
            if (matches == 4) return jackpot * 10 / 100;  // 10% for 4 matches
            if (matches == 3) return jackpot * 5 / 100;   // 5% for 3 matches
            return 0;
        }

        private static void DistributePrizes(BigInteger roundId, BigInteger[] winningNumbers)
        {
            var round = GetRound(roundId);

            // Find all winners and calculate prizes
            for (BigInteger i = 1; i <= round.TicketCount; i++)
            {
                var ticket = GetTicket(roundId, i);
                var matches = CountMatches(ticket.Numbers, winningNumbers);
                var prize = CalculatePrize(round.Jackpot, matches);

                if (prize > 0)
                {
                    // Store winner info
                    var winnerKey = Helper.Concat(new byte[] { PREFIX_WINNER }, (ByteString)roundId);
                    winnerKey = Helper.Concat(winnerKey, (ByteString)i);
                    Storage.Put(Storage.CurrentContext, winnerKey, prize);

                    OnWinnerSelected(roundId, ticket.Buyer, prize, matches);
                }
            }
        }

        public static void OnAutomationCallback(ByteString requestId, ByteString result)
        {
            // Automation trigger registered
        }

        public static void Update(ByteString nefFile, string manifest)
        {
            RequireAdmin();
            ContractManagement.Update(nefFile, manifest, null);
        }
    }

    // ==================== Data Structures ====================

    public class LotteryConfig
    {
        public BigInteger TicketPrice;
        public BigInteger DrawInterval;
        public BigInteger MinTickets;
        public BigInteger[] PrizeTiers;
    }

    public class LotteryRound
    {
        public BigInteger RoundId;
        public BigInteger TicketPrice;
        public BigInteger StartTime;
        public BigInteger EndTime;
        public BigInteger Jackpot;
        public BigInteger TicketCount;
        public byte Status;
        public BigInteger[] WinningNumbers;
        public ByteString VRFRequestId;
        public ByteString AutomationTriggerId;
    }

    public class LotteryTicket
    {
        public BigInteger TicketId;
        public BigInteger RoundId;
        public UInt160 Buyer;
        public BigInteger[] Numbers;
        public BigInteger PurchaseTime;
        public bool Claimed;
    }

    public class VRFPayload
    {
        public ByteString Seed;
        public BigInteger NumWords;
    }

    public class VRFResult
    {
        public ByteString Randomness;
        public ByteString Proof;
    }

    public class AutomationPayload
    {
        public byte TriggerType;
        public UInt160 Target;
        public string Method;
        public ByteString Schedule;
        public BigInteger GasLimit;
        public BigInteger MaxExecutions;
    }
}
