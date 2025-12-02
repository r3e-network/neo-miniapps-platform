using System;
using System.Numerics;
using Neo;
using Neo.SmartContract.Framework;
using Neo.SmartContract.Framework.Attributes;
using Neo.SmartContract.Framework.Native;
using Neo.SmartContract.Framework.Services;

namespace ServiceLayer.Contracts
{
    /// <summary>
    /// MegaLottery - A decentralized lottery contract similar to Mega Millions.
    ///
    /// Features:
    /// - Players pick 5 main numbers (1-70) + 1 mega number (1-25)
    /// - Automated periodic draws via Automation Service
    /// - Verifiable random winning numbers via VRF Service
    /// - Multiple prize tiers based on matches
    /// - Jackpot rollover when no winner
    ///
    /// Integration:
    /// - Uses RandomnessHub for VRF-based winning number generation
    /// - Uses AutomationScheduler for periodic draw execution
    /// - Uses GasBank for prize pool management
    /// </summary>
    [DisplayName("MegaLottery")]
    [ManifestExtra("Author", "R3E Network")]
    [ManifestExtra("Email", "dev@r3e.network")]
    [ManifestExtra("Description", "Decentralized Mega Millions style lottery with VRF and Automation")]
    [ContractPermission("*", "*")]
    public class MegaLottery : SmartContract
    {
        // ============================================================
        // Constants
        // ============================================================

        // Number ranges (Mega Millions style)
        private const int MainNumberMin = 1;
        private const int MainNumberMax = 70;
        private const int MainNumberCount = 5;
        private const int MegaNumberMin = 1;
        private const int MegaNumberMax = 25;

        // Ticket price in GAS (0.1 GAS = 10000000 fractions)
        private static readonly BigInteger TicketPrice = 10_000_000;

        // Prize distribution (percentage of pool)
        private const int JackpotPercent = 50;      // 5+1 match
        private const int SecondPrizePercent = 15;  // 5+0 match
        private const int ThirdPrizePercent = 10;   // 4+1 match
        private const int OperatorPercent = 5;      // Platform fee
        private const int RolloverPercent = 20;     // Next round

        // Draw configuration
        private const int MinTicketsForDraw = 10;
        private const int DrawCooldownBlocks = 100; // ~25 minutes on Neo N3

        // ============================================================
        // Storage
        // ============================================================

        // Contract state
        private static readonly StorageMap Config = new(Storage.CurrentContext, "config:");
        private static readonly StorageMap Rounds = new(Storage.CurrentContext, "round:");
        private static readonly StorageMap Tickets = new(Storage.CurrentContext, "ticket:");
        private static readonly StorageMap PlayerTickets = new(Storage.CurrentContext, "player:");
        private static readonly StorageMap Claims = new(Storage.CurrentContext, "claim:");
        private static readonly StorageMap VRFRequests = new(Storage.CurrentContext, "vrf:");

        // Config keys
        private const string KeyOwner = "owner";
        private const string KeyOperator = "operator";
        private const string KeyCurrentRound = "currentRound";
        private const string KeyTotalPrizePool = "totalPool";
        private const string KeyRandomnessHub = "randomnessHub";
        private const string KeyAutomationJob = "automationJob";
        private const string KeyVRFKeyId = "vrfKeyId";
        private const string KeyPaused = "paused";

        // ============================================================
        // Events
        // ============================================================

        public static event Action<UInt160, BigInteger, ByteString> TicketPurchased;      // player, roundId, ticketId
        public static event Action<BigInteger, byte[], byte> DrawInitiated;               // roundId, requestId, status
        public static event Action<BigInteger, byte[], byte, BigInteger> DrawCompleted;   // roundId, winningNumbers, megaNumber, jackpot
        public static event Action<UInt160, BigInteger, BigInteger, int> PrizeClaimed;    // player, roundId, amount, tier
        public static event Action<BigInteger, BigInteger> RoundStarted;                  // roundId, startBlock
        public static event Action<BigInteger, BigInteger> JackpotRollover;               // fromRound, amount

        // ============================================================
        // Data Structures
        // ============================================================

        public struct LotteryRound
        {
            public BigInteger RoundId;
            public BigInteger StartBlock;
            public BigInteger EndBlock;
            public BigInteger PrizePool;
            public BigInteger TicketCount;
            public byte[] WinningNumbers;    // 5 main numbers
            public byte MegaNumber;
            public byte Status;              // 0=active, 1=drawing, 2=completed, 3=cancelled
            public ByteString VRFRequestId;
            public BigInteger JackpotWinners;
            public BigInteger SecondWinners;
            public BigInteger ThirdWinners;
        }

        public struct Ticket
        {
            public ByteString TicketId;
            public UInt160 Player;
            public BigInteger RoundId;
            public byte[] Numbers;           // 5 main numbers
            public byte MegaNumber;
            public BigInteger PurchaseBlock;
            public bool Claimed;
        }

        // Round status
        public const byte StatusActive = 0;
        public const byte StatusDrawing = 1;
        public const byte StatusCompleted = 2;
        public const byte StatusCancelled = 3;

        // Prize tiers
        public const int TierJackpot = 1;    // 5+1
        public const int TierSecond = 2;     // 5+0
        public const int TierThird = 3;      // 4+1
        public const int TierFourth = 4;     // 4+0
        public const int TierFifth = 5;      // 3+1
        public const int TierSixth = 6;      // 3+0
        public const int TierSeventh = 7;    // 2+1
        public const int TierEighth = 8;     // 1+1
        public const int TierNinth = 9;      // 0+1

        // ============================================================
        // Contract Lifecycle
        // ============================================================

        [Safe]
        public static string Name() => "MegaLottery";

        [Safe]
        public static string Symbol() => "MLOT";

        public static void _deploy(object data, bool update)
        {
            if (update) return;

            var tx = (Transaction)Runtime.ScriptContainer;
            Config.Put(KeyOwner, tx.Sender);
            Config.Put(KeyOperator, tx.Sender);
            Config.Put(KeyCurrentRound, 0);
            Config.Put(KeyTotalPrizePool, 0);
            Config.Put(KeyPaused, 0);

            // Start first round
            StartNewRound();
        }

        public static void Update(ByteString nefFile, string manifest)
        {
            ValidateOwner();
            ContractManagement.Update(nefFile, manifest);
        }

        // ============================================================
        // Configuration
        // ============================================================

        public static void SetRandomnessHub(UInt160 hub)
        {
            ValidateOwner();
            Config.Put(KeyRandomnessHub, hub);
        }

        public static void SetVRFKeyId(ByteString keyId)
        {
            ValidateOwner();
            Config.Put(KeyVRFKeyId, keyId);
        }

        public static void SetAutomationJob(ByteString jobId)
        {
            ValidateOwner();
            Config.Put(KeyAutomationJob, jobId);
        }

        public static void SetOperator(UInt160 newOperator)
        {
            ValidateOwner();
            Config.Put(KeyOperator, newOperator);
        }

        public static void SetPaused(bool paused)
        {
            ValidateOwner();
            Config.Put(KeyPaused, paused ? 1 : 0);
        }

        [Safe]
        public static UInt160 GetOwner() => (UInt160)Config.Get(KeyOwner);

        [Safe]
        public static UInt160 GetOperator() => (UInt160)Config.Get(KeyOperator);

        [Safe]
        public static BigInteger GetCurrentRound() => (BigInteger)Config.Get(KeyCurrentRound);

        [Safe]
        public static BigInteger GetTotalPrizePool() => (BigInteger)Config.Get(KeyTotalPrizePool);

        [Safe]
        public static bool IsPaused() => (BigInteger)Config.Get(KeyPaused) == 1;

        // ============================================================
        // Ticket Purchase
        // ============================================================

        /// <summary>
        /// Purchase a lottery ticket with chosen numbers.
        /// </summary>
        /// <param name="numbers">5 main numbers (1-70)</param>
        /// <param name="megaNumber">1 mega number (1-25)</param>
        public static ByteString BuyTicket(byte[] numbers, byte megaNumber)
        {
            Assert(!IsPaused(), "Contract is paused");

            var tx = (Transaction)Runtime.ScriptContainer;
            var player = tx.Sender;

            // Validate numbers
            Assert(numbers.Length == MainNumberCount, "Must provide exactly 5 main numbers");
            Assert(megaNumber >= MegaNumberMin && megaNumber <= MegaNumberMax, "Mega number must be 1-25");

            // Validate main numbers are unique and in range
            for (int i = 0; i < MainNumberCount; i++)
            {
                Assert(numbers[i] >= MainNumberMin && numbers[i] <= MainNumberMax, "Main numbers must be 1-70");
                for (int j = i + 1; j < MainNumberCount; j++)
                {
                    Assert(numbers[i] != numbers[j], "Main numbers must be unique");
                }
            }

            // Get current round
            BigInteger roundId = GetCurrentRound();
            LotteryRound round = GetRound(roundId);
            Assert(round.Status == StatusActive, "Round is not active");

            // Transfer ticket price
            Assert(GAS.Transfer(player, Runtime.ExecutingScriptHash, TicketPrice), "Payment failed");

            // Generate ticket ID
            ByteString ticketId = CryptoLib.Sha256(
                Runtime.ExecutingScriptHash.ToByteString() +
                (ByteString)roundId +
                (ByteString)round.TicketCount +
                (ByteString)Runtime.GetRandom()
            );

            // Create ticket
            Ticket ticket = new Ticket
            {
                TicketId = ticketId,
                Player = player,
                RoundId = roundId,
                Numbers = numbers,
                MegaNumber = megaNumber,
                PurchaseBlock = Ledger.CurrentIndex,
                Claimed = false
            };

            // Store ticket
            Tickets.Put(ticketId, StdLib.Serialize(ticket));

            // Add to player's tickets
            ByteString playerKey = player.ToByteString() + (ByteString)roundId;
            ByteString existingTickets = PlayerTickets.Get(playerKey);
            if (existingTickets is null)
            {
                PlayerTickets.Put(playerKey, ticketId);
            }
            else
            {
                PlayerTickets.Put(playerKey, existingTickets + ticketId);
            }

            // Update round
            round.TicketCount += 1;
            round.PrizePool += TicketPrice;
            SaveRound(round);

            // Update total pool
            BigInteger totalPool = GetTotalPrizePool();
            Config.Put(KeyTotalPrizePool, totalPool + TicketPrice);

            TicketPurchased(player, roundId, ticketId);

            return ticketId;
        }

        /// <summary>
        /// Quick pick - generate random numbers for the player.
        /// </summary>
        public static ByteString QuickPick()
        {
            byte[] numbers = new byte[MainNumberCount];

            // Generate 5 unique random main numbers
            for (int i = 0; i < MainNumberCount; i++)
            {
                byte num;
                bool unique;
                do
                {
                    num = (byte)(Runtime.GetRandom() % MainNumberMax + MainNumberMin);
                    unique = true;
                    for (int j = 0; j < i; j++)
                    {
                        if (numbers[j] == num)
                        {
                            unique = false;
                            break;
                        }
                    }
                } while (!unique);
                numbers[i] = num;
            }

            // Generate mega number
            byte megaNumber = (byte)(Runtime.GetRandom() % MegaNumberMax + MegaNumberMin);

            return BuyTicket(numbers, megaNumber);
        }

        // ============================================================
        // Draw Execution (Called by Automation Service)
        // ============================================================

        /// <summary>
        /// Initiate the draw process. Called by automation job.
        /// Requests random numbers from VRF service.
        /// </summary>
        public static void InitiateDraw()
        {
            ValidateOperatorOrAutomation();

            BigInteger roundId = GetCurrentRound();
            LotteryRound round = GetRound(roundId);

            Assert(round.Status == StatusActive, "Round is not active");
            Assert(round.TicketCount >= MinTicketsForDraw, "Not enough tickets sold");

            // Update round status
            round.Status = StatusDrawing;
            round.EndBlock = Ledger.CurrentIndex;
            SaveRound(round);

            // Request randomness from VRF
            UInt160 randomnessHub = (UInt160)Config.Get(KeyRandomnessHub);
            Assert(randomnessHub is not null, "RandomnessHub not configured");

            ByteString vrfKeyId = Config.Get(KeyVRFKeyId);
            Assert(vrfKeyId is not null, "VRF Key not configured");

            // Create seed from round data
            ByteString seed = CryptoLib.Sha256(
                (ByteString)roundId +
                (ByteString)round.TicketCount +
                (ByteString)round.PrizePool +
                (ByteString)Ledger.CurrentIndex
            );

            // Request 6 random numbers (5 main + 1 mega)
            object[] args = new object[] { seed, 6, 3600, Runtime.ExecutingScriptHash, "FulfillDraw" };
            ByteString requestId = (ByteString)Contract.Call(randomnessHub, "RequestRandomness", CallFlags.All, args);

            // Store VRF request mapping
            VRFRequests.Put(requestId, roundId);
            round.VRFRequestId = requestId;
            SaveRound(round);

            DrawInitiated(roundId, (byte[])requestId, StatusDrawing);
        }

        /// <summary>
        /// Callback from VRF service with random numbers.
        /// </summary>
        public static void FulfillDraw(ByteString requestId, ByteString randomOutput)
        {
            // Verify caller is RandomnessHub
            UInt160 randomnessHub = (UInt160)Config.Get(KeyRandomnessHub);
            Assert(Runtime.CallingScriptHash == randomnessHub, "Only RandomnessHub can fulfill");

            // Get round from request
            BigInteger roundId = (BigInteger)VRFRequests.Get(requestId);
            Assert(roundId > 0, "Unknown VRF request");

            LotteryRound round = GetRound(roundId);
            Assert(round.Status == StatusDrawing, "Round is not in drawing state");

            // Parse random output to generate winning numbers
            byte[] randomBytes = (byte[])randomOutput;
            byte[] winningNumbers = new byte[MainNumberCount];

            // Generate 5 unique main numbers from random bytes
            int usedBytes = 0;
            for (int i = 0; i < MainNumberCount; i++)
            {
                byte num;
                bool unique;
                do
                {
                    // Use modulo to get number in range
                    num = (byte)(randomBytes[usedBytes % randomBytes.Length] % MainNumberMax + MainNumberMin);
                    usedBytes++;
                    unique = true;
                    for (int j = 0; j < i; j++)
                    {
                        if (winningNumbers[j] == num)
                        {
                            unique = false;
                            break;
                        }
                    }
                } while (!unique && usedBytes < randomBytes.Length * 2);
                winningNumbers[i] = num;
            }

            // Generate mega number
            byte megaNumber = (byte)(randomBytes[usedBytes % randomBytes.Length] % MegaNumberMax + MegaNumberMin);

            // Update round with winning numbers
            round.WinningNumbers = winningNumbers;
            round.MegaNumber = megaNumber;
            round.Status = StatusCompleted;
            SaveRound(round);

            // Calculate prize distribution
            DistributePrizes(round);

            DrawCompleted(roundId, winningNumbers, megaNumber, round.PrizePool);

            // Start new round
            StartNewRound();
        }

        // ============================================================
        // Prize Distribution
        // ============================================================

        private static void DistributePrizes(LotteryRound round)
        {
            BigInteger prizePool = round.PrizePool;

            // Calculate prize amounts
            BigInteger jackpotPool = prizePool * JackpotPercent / 100;
            BigInteger secondPool = prizePool * SecondPrizePercent / 100;
            BigInteger thirdPool = prizePool * ThirdPrizePercent / 100;
            BigInteger operatorFee = prizePool * OperatorPercent / 100;
            BigInteger rollover = prizePool * RolloverPercent / 100;

            // Transfer operator fee
            UInt160 operatorAddr = GetOperator();
            if (operatorFee > 0)
            {
                GAS.Transfer(Runtime.ExecutingScriptHash, operatorAddr, operatorFee);
            }

            // If no jackpot winner, rollover to next round
            if (round.JackpotWinners == 0)
            {
                rollover += jackpotPool;
                JackpotRollover(round.RoundId, jackpotPool);
            }

            // Store rollover for next round
            Config.Put("rollover", rollover);
        }

        /// <summary>
        /// Claim prize for a winning ticket.
        /// </summary>
        public static BigInteger ClaimPrize(ByteString ticketId)
        {
            Ticket ticket = GetTicket(ticketId);
            Assert(!ticket.Claimed, "Prize already claimed");
            Assert(Runtime.CheckWitness(ticket.Player), "Not ticket owner");

            LotteryRound round = GetRound(ticket.RoundId);
            Assert(round.Status == StatusCompleted, "Round not completed");

            // Calculate matches
            int mainMatches = CountMatches(ticket.Numbers, round.WinningNumbers);
            bool megaMatch = ticket.MegaNumber == round.MegaNumber;

            // Determine prize tier and amount
            int tier = GetPrizeTier(mainMatches, megaMatch);
            BigInteger prizeAmount = CalculatePrize(round, tier, mainMatches, megaMatch);

            if (prizeAmount > 0)
            {
                // Mark as claimed
                ticket.Claimed = true;
                Tickets.Put(ticketId, StdLib.Serialize(ticket));

                // Transfer prize
                Assert(GAS.Transfer(Runtime.ExecutingScriptHash, ticket.Player, prizeAmount), "Prize transfer failed");

                PrizeClaimed(ticket.Player, ticket.RoundId, prizeAmount, tier);
            }

            return prizeAmount;
        }

        private static int CountMatches(byte[] playerNumbers, byte[] winningNumbers)
        {
            int matches = 0;
            for (int i = 0; i < playerNumbers.Length; i++)
            {
                for (int j = 0; j < winningNumbers.Length; j++)
                {
                    if (playerNumbers[i] == winningNumbers[j])
                    {
                        matches++;
                        break;
                    }
                }
            }
            return matches;
        }

        private static int GetPrizeTier(int mainMatches, bool megaMatch)
        {
            if (mainMatches == 5 && megaMatch) return TierJackpot;
            if (mainMatches == 5) return TierSecond;
            if (mainMatches == 4 && megaMatch) return TierThird;
            if (mainMatches == 4) return TierFourth;
            if (mainMatches == 3 && megaMatch) return TierFifth;
            if (mainMatches == 3) return TierSixth;
            if (mainMatches == 2 && megaMatch) return TierSeventh;
            if (mainMatches == 1 && megaMatch) return TierEighth;
            if (megaMatch) return TierNinth;
            return 0; // No prize
        }

        private static BigInteger CalculatePrize(LotteryRound round, int tier, int mainMatches, bool megaMatch)
        {
            BigInteger prizePool = round.PrizePool;

            // Fixed prizes for lower tiers (in GAS fractions)
            switch (tier)
            {
                case TierJackpot:
                    return round.JackpotWinners > 0
                        ? prizePool * JackpotPercent / 100 / round.JackpotWinners
                        : 0;
                case TierSecond:
                    return round.SecondWinners > 0
                        ? prizePool * SecondPrizePercent / 100 / round.SecondWinners
                        : 0;
                case TierThird:
                    return round.ThirdWinners > 0
                        ? prizePool * ThirdPrizePercent / 100 / round.ThirdWinners
                        : 0;
                case TierFourth:
                    return 50_000_000;  // 0.5 GAS
                case TierFifth:
                    return 20_000_000;  // 0.2 GAS
                case TierSixth:
                    return 10_000_000;  // 0.1 GAS
                case TierSeventh:
                    return 5_000_000;   // 0.05 GAS
                case TierEighth:
                    return 2_000_000;   // 0.02 GAS
                case TierNinth:
                    return 1_000_000;   // 0.01 GAS
                default:
                    return 0;
            }
        }

        // ============================================================
        // Round Management
        // ============================================================

        private static void StartNewRound()
        {
            BigInteger newRoundId = GetCurrentRound() + 1;
            BigInteger rollover = (BigInteger)Config.Get("rollover");
            if (rollover is null) rollover = 0;

            LotteryRound round = new LotteryRound
            {
                RoundId = newRoundId,
                StartBlock = Ledger.CurrentIndex,
                EndBlock = 0,
                PrizePool = rollover,
                TicketCount = 0,
                WinningNumbers = new byte[0],
                MegaNumber = 0,
                Status = StatusActive,
                VRFRequestId = null,
                JackpotWinners = 0,
                SecondWinners = 0,
                ThirdWinners = 0
            };

            SaveRound(round);
            Config.Put(KeyCurrentRound, newRoundId);
            Config.Put("rollover", 0);

            RoundStarted(newRoundId, round.StartBlock);
        }

        [Safe]
        public static LotteryRound GetRound(BigInteger roundId)
        {
            ByteString data = Rounds.Get((ByteString)roundId);
            if (data is null) return new LotteryRound();
            return (LotteryRound)StdLib.Deserialize(data);
        }

        private static void SaveRound(LotteryRound round)
        {
            Rounds.Put((ByteString)round.RoundId, StdLib.Serialize(round));
        }

        [Safe]
        public static Ticket GetTicket(ByteString ticketId)
        {
            ByteString data = Tickets.Get(ticketId);
            if (data is null) throw new Exception("Ticket not found");
            return (Ticket)StdLib.Deserialize(data);
        }

        [Safe]
        public static ByteString[] GetPlayerTickets(UInt160 player, BigInteger roundId)
        {
            ByteString playerKey = player.ToByteString() + (ByteString)roundId;
            ByteString ticketIds = PlayerTickets.Get(playerKey);
            if (ticketIds is null) return new ByteString[0];

            // Each ticket ID is 32 bytes (SHA256)
            int count = ticketIds.Length / 32;
            ByteString[] result = new ByteString[count];
            for (int i = 0; i < count; i++)
            {
                result[i] = ticketIds.Range(i * 32, 32);
            }
            return result;
        }

        // ============================================================
        // Validation Helpers
        // ============================================================

        private static void ValidateOwner()
        {
            UInt160 owner = GetOwner();
            Assert(Runtime.CheckWitness(owner), "Not authorized: owner only");
        }

        private static void ValidateOperatorOrAutomation()
        {
            UInt160 operatorAddr = GetOperator();
            ByteString automationJob = Config.Get(KeyAutomationJob);

            bool isOperator = Runtime.CheckWitness(operatorAddr);
            bool isAutomation = automationJob is not null &&
                Runtime.CallingScriptHash == (UInt160)automationJob;

            Assert(isOperator || isAutomation, "Not authorized: operator or automation only");
        }

        private static void Assert(bool condition, string message)
        {
            if (!condition) throw new Exception(message);
        }
    }
}
