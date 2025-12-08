using System;
using System.ComponentModel;
using System.Numerics;
using Neo;
using Neo.SmartContract;
using Neo.SmartContract.Framework;
using Neo.SmartContract.Framework.Attributes;
using Neo.SmartContract.Framework.Native;
using Neo.SmartContract.Framework.Services;

namespace MegaLottery
{
    /// <summary>
    /// MegaLottery - A decentralized lottery powered by Service Layer
    ///
    /// Features:
    /// - Daily draws using VRF for provably fair random numbers
    /// - Automation service triggers daily unveil events
    /// - Pick 5 numbers (1-70) + 1 Mega number (1-25)
    /// - Multiple prize tiers based on matches
    /// - 1-minute purchase lockout before draws
    /// </summary>
    [DisplayName("MegaLottery")]
    [ManifestExtra("Author", "Service Layer Team")]
    [ManifestExtra("Email", "dev@servicelayer.io")]
    [ManifestExtra("Description", "Decentralized MegaMillions-style lottery powered by Service Layer VRF")]
    [ContractPermission("*", "*")]
    public class MegaLottery : SmartContract
    {
        // =====================================================================
        // Constants
        // =====================================================================

        private const byte MAIN_NUMBERS_COUNT = 5;      // Pick 5 main numbers
        private const byte MAIN_NUMBER_MAX = 70;        // Main numbers: 1-70
        private const byte MEGA_NUMBER_MAX = 25;        // Mega number: 1-25
        private const ulong TICKET_PRICE = 200000000;   // 2 GAS per ticket
        private const ulong LOCKOUT_PERIOD = 60000;     // 1 minute before draw (in ms)
        private const ulong DRAW_INTERVAL = 86400000;   // 24 hours (in ms)

        // Prize distribution (percentage of pool)
        private const byte JACKPOT_PERCENT = 50;        // 5+1 match
        private const byte SECOND_PERCENT = 20;         // 5+0 match
        private const byte THIRD_PERCENT = 10;          // 4+1 match
        private const byte FOURTH_PERCENT = 10;         // 4+0 or 3+1 match
        private const byte OPERATIONS_PERCENT = 10;     // Operations fund

        // Storage prefixes
        private static readonly byte[] PREFIX_ADMIN = new byte[] { 0x01 };
        private static readonly byte[] PREFIX_DRAW = new byte[] { 0x02 };
        private static readonly byte[] PREFIX_TICKET = new byte[] { 0x03 };
        private static readonly byte[] PREFIX_USER_TICKETS = new byte[] { 0x04 };
        private static readonly byte[] PREFIX_POOL = new byte[] { 0x05 };
        private static readonly byte[] PREFIX_CONFIG = new byte[] { 0x06 };
        private static readonly byte[] PREFIX_VRF_REQUEST = new byte[] { 0x07 };
        private static readonly byte[] PREFIX_WINNERS = new byte[] { 0x08 };

        // Config keys
        private static readonly byte[] KEY_VRF_CONTRACT = new byte[] { 0x01 };
        private static readonly byte[] KEY_AUTOMATION_CONTRACT = new byte[] { 0x02 };
        private static readonly byte[] KEY_CURRENT_DRAW = new byte[] { 0x03 };
        private static readonly byte[] KEY_NEXT_DRAW_TIME = new byte[] { 0x04 };
        private static readonly byte[] KEY_TOTAL_TICKETS = new byte[] { 0x05 };
        private static readonly byte[] KEY_JACKPOT = new byte[] { 0x06 };
        private static readonly byte[] KEY_PAUSED = new byte[] { 0x07 };
        private static readonly byte[] KEY_TRIGGER_ID = new byte[] { 0x08 };

        // =====================================================================
        // Events
        // =====================================================================

        [DisplayName("TicketPurchased")]
        public static event Action<UInt160, BigInteger, BigInteger, byte[], byte> OnTicketPurchased;

        [DisplayName("DrawStarted")]
        public static event Action<BigInteger, BigInteger> OnDrawStarted;

        [DisplayName("DrawCompleted")]
        public static event Action<BigInteger, byte[], byte> OnDrawCompleted;

        [DisplayName("PrizeClaimed")]
        public static event Action<UInt160, BigInteger, BigInteger, byte> OnPrizeClaimed;

        [DisplayName("JackpotRollover")]
        public static event Action<BigInteger, BigInteger> OnJackpotRollover;

        // =====================================================================
        // Data Structures
        // =====================================================================

        public struct Ticket
        {
            public BigInteger TicketId;
            public BigInteger DrawId;
            public UInt160 Owner;
            public byte[] MainNumbers;  // 5 numbers (1-70)
            public byte MegaNumber;     // 1 number (1-25)
            public ulong PurchaseTime;
            public bool Claimed;
            public byte PrizeTier;      // 0=no win, 1=jackpot, 2=second, etc.
        }

        public struct Draw
        {
            public BigInteger DrawId;
            public ulong StartTime;
            public ulong DrawTime;
            public byte[] WinningNumbers;   // 5 main numbers
            public byte WinningMega;        // Mega number
            public BigInteger TotalPool;
            public BigInteger TicketCount;
            public bool Completed;
            public ByteString VrfRequestId;
            public BigInteger[] WinnerCounts;  // Count per tier
            public BigInteger[] PrizeAmounts;  // Prize per tier
        }

        // =====================================================================
        // Contract Lifecycle
        // =====================================================================

        [DisplayName("_deploy")]
        public static void Deploy(object data, bool update)
        {
            if (update) return;

            var tx = (Transaction)Runtime.ScriptContainer;
            Storage.Put(Storage.CurrentContext, PREFIX_ADMIN, tx.Sender);

            // Initialize first draw
            BigInteger drawId = 1;
            Storage.Put(Storage.CurrentContext, PREFIX_CONFIG.Concat(KEY_CURRENT_DRAW), drawId);
            Storage.Put(Storage.CurrentContext, PREFIX_CONFIG.Concat(KEY_TOTAL_TICKETS), 0);
            Storage.Put(Storage.CurrentContext, PREFIX_CONFIG.Concat(KEY_JACKPOT), 0);
            Storage.Put(Storage.CurrentContext, PREFIX_CONFIG.Concat(KEY_PAUSED), 0);

            // Set next draw time to tomorrow at 00:00 UTC
            ulong nextDraw = GetNextDrawTime();
            Storage.Put(Storage.CurrentContext, PREFIX_CONFIG.Concat(KEY_NEXT_DRAW_TIME), nextDraw);

            // Initialize first draw record
            InitializeDraw(drawId, nextDraw);
        }

        public static void Update(ByteString nefFile, string manifest)
        {
            ValidateAdmin();
            ContractManagement.Update(nefFile, manifest);
        }

        // =====================================================================
        // Admin Functions
        // =====================================================================

        public static void SetVRFContract(UInt160 vrfContract)
        {
            ValidateAdmin();
            Storage.Put(Storage.CurrentContext, PREFIX_CONFIG.Concat(KEY_VRF_CONTRACT), vrfContract);
        }

        public static void SetAutomationContract(UInt160 automationContract)
        {
            ValidateAdmin();
            Storage.Put(Storage.CurrentContext, PREFIX_CONFIG.Concat(KEY_AUTOMATION_CONTRACT), automationContract);
        }

        public static void RegisterDailyTrigger()
        {
            ValidateAdmin();

            UInt160 automationContract = GetAutomationContract();
            ExecutionEngine.Assert(automationContract != null, "Automation contract not set");

            // Register time-based trigger for daily draws
            // Trigger type 1 = Time-based
            // Condition: Cron expression for daily at 00:00 UTC
            object[] args = new object[] {
                Runtime.ExecutingScriptHash,    // Callback contract
                (byte)1,                        // Trigger type: Time
                "0 0 * * *",                    // Cron: Daily at midnight
                "unveilWinner",                 // Callback method
                0                               // Max executions (0 = unlimited)
            };

            BigInteger triggerId = (BigInteger)Contract.Call(automationContract, "registerTrigger", CallFlags.All, args);
            Storage.Put(Storage.CurrentContext, PREFIX_CONFIG.Concat(KEY_TRIGGER_ID), triggerId);
        }

        public static void Pause()
        {
            ValidateAdmin();
            Storage.Put(Storage.CurrentContext, PREFIX_CONFIG.Concat(KEY_PAUSED), 1);
        }

        public static void Unpause()
        {
            ValidateAdmin();
            Storage.Put(Storage.CurrentContext, PREFIX_CONFIG.Concat(KEY_PAUSED), 0);
        }

        public static void WithdrawOperationsFund(UInt160 to, BigInteger amount)
        {
            ValidateAdmin();
            ExecutionEngine.Assert(Runtime.CheckWitness(to), "Invalid witness");

            // Transfer GAS to operations address
            GAS.Transfer(Runtime.ExecutingScriptHash, to, amount);
        }

        // =====================================================================
        // Public Functions - Ticket Purchase
        // =====================================================================

        /// <summary>
        /// Purchase a lottery ticket with chosen numbers
        /// </summary>
        /// <param name="mainNumbers">5 numbers between 1-70</param>
        /// <param name="megaNumber">1 number between 1-25</param>
        public static BigInteger BuyTicket(byte[] mainNumbers, byte megaNumber)
        {
            ExecutionEngine.Assert(!IsPaused(), "Contract is paused");
            ExecutionEngine.Assert(!IsInLockoutPeriod(), "Ticket sales locked before draw");

            // Validate numbers
            ExecutionEngine.Assert(mainNumbers.Length == MAIN_NUMBERS_COUNT, "Must pick 5 main numbers");
            ExecutionEngine.Assert(megaNumber >= 1 && megaNumber <= MEGA_NUMBER_MAX, "Mega number must be 1-25");

            // Validate main numbers are unique and in range
            for (int i = 0; i < MAIN_NUMBERS_COUNT; i++)
            {
                ExecutionEngine.Assert(mainNumbers[i] >= 1 && mainNumbers[i] <= MAIN_NUMBER_MAX,
                    "Main numbers must be 1-70");
                for (int j = i + 1; j < MAIN_NUMBERS_COUNT; j++)
                {
                    ExecutionEngine.Assert(mainNumbers[i] != mainNumbers[j], "Numbers must be unique");
                }
            }

            // Get buyer
            var tx = (Transaction)Runtime.ScriptContainer;
            UInt160 buyer = tx.Sender;

            // Transfer ticket price
            bool transferred = GAS.Transfer(buyer, Runtime.ExecutingScriptHash, TICKET_PRICE);
            ExecutionEngine.Assert(transferred, "Payment failed");

            // Get current draw
            BigInteger currentDraw = GetCurrentDrawId();

            // Create ticket
            BigInteger ticketId = GetAndIncrementTicketCounter();

            Ticket ticket = new Ticket
            {
                TicketId = ticketId,
                DrawId = currentDraw,
                Owner = buyer,
                MainNumbers = SortNumbers(mainNumbers),
                MegaNumber = megaNumber,
                PurchaseTime = Runtime.Time,
                Claimed = false,
                PrizeTier = 0
            };

            // Store ticket
            SaveTicket(ticket);

            // Add to user's tickets
            AddUserTicket(buyer, ticketId);

            // Update draw pool
            UpdateDrawPool(currentDraw, TICKET_PRICE);

            // Emit event
            OnTicketPurchased(buyer, ticketId, currentDraw, ticket.MainNumbers, megaNumber);

            return ticketId;
        }

        /// <summary>
        /// Quick pick - generate random numbers for the user
        /// Uses block hash as entropy source for number selection
        /// </summary>
        public static BigInteger QuickPick()
        {
            // Generate pseudo-random numbers using block data
            ByteString entropy = Runtime.GetRandom().ToByteString();

            byte[] mainNumbers = new byte[MAIN_NUMBERS_COUNT];
            int index = 0;
            int attempts = 0;

            while (index < MAIN_NUMBERS_COUNT && attempts < 100)
            {
                byte num = (byte)((entropy[attempts % entropy.Length] % MAIN_NUMBER_MAX) + 1);
                bool duplicate = false;

                for (int i = 0; i < index; i++)
                {
                    if (mainNumbers[i] == num)
                    {
                        duplicate = true;
                        break;
                    }
                }

                if (!duplicate)
                {
                    mainNumbers[index] = num;
                    index++;
                }
                attempts++;
            }

            byte megaNumber = (byte)((entropy[5] % MEGA_NUMBER_MAX) + 1);

            return BuyTicket(mainNumbers, megaNumber);
        }

        // =====================================================================
        // Draw Functions - Called by Automation Service
        // =====================================================================

        /// <summary>
        /// Called by Automation service to start the draw process
        /// Requests VRF random numbers
        /// </summary>
        public static void UnveilWinner()
        {
            // Can be called by automation contract or admin
            UInt160 automationContract = GetAutomationContract();
            bool isAutomation = automationContract != null && Runtime.CallingScriptHash == automationContract;
            bool isAdmin = IsAdmin();

            ExecutionEngine.Assert(isAutomation || isAdmin, "Unauthorized");

            BigInteger currentDraw = GetCurrentDrawId();
            Draw draw = GetDraw(currentDraw);

            ExecutionEngine.Assert(!draw.Completed, "Draw already completed");
            ExecutionEngine.Assert(Runtime.Time >= draw.DrawTime, "Draw time not reached");

            // Request VRF random numbers
            UInt160 vrfContract = GetVRFContract();
            ExecutionEngine.Assert(vrfContract != null, "VRF contract not set");

            // Request 6 random numbers (5 main + 1 mega)
            ByteString seed = Runtime.ExecutingScriptHash.ToByteString() + currentDraw.ToByteString();

            object[] args = new object[] {
                seed,                           // Seed
                6,                              // Number of random words
                Runtime.ExecutingScriptHash,    // Callback contract
                200000                          // Callback gas limit
            };

            ByteString requestId = (ByteString)Contract.Call(vrfContract, "requestRandomness", CallFlags.All, args);

            // Store VRF request ID
            Storage.Put(Storage.CurrentContext, PREFIX_VRF_REQUEST.Concat(currentDraw.ToByteArray()), requestId);

            OnDrawStarted(currentDraw, Runtime.Time);
        }

        /// <summary>
        /// VRF callback - receives random numbers and completes the draw
        /// </summary>
        public static void FulfillRandomness(ByteString requestId, BigInteger[] randomWords)
        {
            // Verify caller is VRF contract
            UInt160 vrfContract = GetVRFContract();
            ExecutionEngine.Assert(Runtime.CallingScriptHash == vrfContract, "Only VRF contract can call");

            BigInteger currentDraw = GetCurrentDrawId();
            ByteString storedRequestId = Storage.Get(Storage.CurrentContext,
                PREFIX_VRF_REQUEST.Concat(currentDraw.ToByteArray()));

            ExecutionEngine.Assert(requestId == storedRequestId, "Invalid request ID");

            // Convert random words to winning numbers
            byte[] winningNumbers = new byte[MAIN_NUMBERS_COUNT];

            // Generate unique main numbers from random words
            for (int i = 0; i < MAIN_NUMBERS_COUNT; i++)
            {
                BigInteger rand = randomWords[i];
                byte num;
                int attempts = 0;

                do
                {
                    num = (byte)((rand % MAIN_NUMBER_MAX) + 1);
                    rand = rand / MAIN_NUMBER_MAX + attempts;
                    attempts++;
                } while (ContainsNumber(winningNumbers, num, i) && attempts < 100);

                winningNumbers[i] = num;
            }

            // Generate mega number
            byte winningMega = (byte)((randomWords[5] % MEGA_NUMBER_MAX) + 1);

            // Sort winning numbers
            winningNumbers = SortNumbers(winningNumbers);

            // Complete the draw
            CompleteDraw(currentDraw, winningNumbers, winningMega);
        }

        // =====================================================================
        // Prize Claiming
        // =====================================================================

        /// <summary>
        /// Check and claim prize for a ticket
        /// </summary>
        public static BigInteger ClaimPrize(BigInteger ticketId)
        {
            Ticket ticket = GetTicket(ticketId);
            ExecutionEngine.Assert(ticket.TicketId > 0, "Ticket not found");
            ExecutionEngine.Assert(!ticket.Claimed, "Prize already claimed");
            ExecutionEngine.Assert(Runtime.CheckWitness(ticket.Owner), "Not ticket owner");

            Draw draw = GetDraw(ticket.DrawId);
            ExecutionEngine.Assert(draw.Completed, "Draw not completed");

            // Calculate prize tier
            byte tier = CalculatePrizeTier(ticket.MainNumbers, ticket.MegaNumber,
                draw.WinningNumbers, draw.WinningMega);

            if (tier == 0)
            {
                return 0; // No prize
            }

            // Get prize amount for this tier
            BigInteger prizeAmount = draw.PrizeAmounts[tier - 1];
            ExecutionEngine.Assert(prizeAmount > 0, "No prize for this tier");

            // Mark ticket as claimed
            ticket.Claimed = true;
            ticket.PrizeTier = tier;
            SaveTicket(ticket);

            // Transfer prize
            GAS.Transfer(Runtime.ExecutingScriptHash, ticket.Owner, prizeAmount);

            OnPrizeClaimed(ticket.Owner, ticketId, prizeAmount, tier);

            return prizeAmount;
        }

        /// <summary>
        /// Check prize tier for a ticket without claiming
        /// </summary>
        public static byte CheckTicket(BigInteger ticketId)
        {
            Ticket ticket = GetTicket(ticketId);
            ExecutionEngine.Assert(ticket.TicketId > 0, "Ticket not found");

            Draw draw = GetDraw(ticket.DrawId);
            if (!draw.Completed)
            {
                return 0; // Draw not completed yet
            }

            return CalculatePrizeTier(ticket.MainNumbers, ticket.MegaNumber,
                draw.WinningNumbers, draw.WinningMega);
        }

        // =====================================================================
        // View Functions
        // =====================================================================

        public static BigInteger GetCurrentDrawId()
        {
            return (BigInteger)Storage.Get(Storage.CurrentContext, PREFIX_CONFIG.Concat(KEY_CURRENT_DRAW));
        }

        public static ulong GetNextDrawTime()
        {
            // Calculate next midnight UTC
            ulong now = Runtime.Time;
            ulong dayMs = 86400000;
            ulong nextMidnight = ((now / dayMs) + 1) * dayMs;
            return nextMidnight;
        }

        public static Draw GetDraw(BigInteger drawId)
        {
            ByteString data = Storage.Get(Storage.CurrentContext, PREFIX_DRAW.Concat(drawId.ToByteArray()));
            if (data == null) return new Draw();
            return (Draw)StdLib.Deserialize(data);
        }

        public static Ticket GetTicket(BigInteger ticketId)
        {
            ByteString data = Storage.Get(Storage.CurrentContext, PREFIX_TICKET.Concat(ticketId.ToByteArray()));
            if (data == null) return new Ticket();
            return (Ticket)StdLib.Deserialize(data);
        }

        public static BigInteger[] GetUserTickets(UInt160 user)
        {
            ByteString data = Storage.Get(Storage.CurrentContext, PREFIX_USER_TICKETS.Concat(user));
            if (data == null) return new BigInteger[0];
            return (BigInteger[])StdLib.Deserialize(data);
        }

        public static BigInteger GetJackpot()
        {
            return (BigInteger)Storage.Get(Storage.CurrentContext, PREFIX_CONFIG.Concat(KEY_JACKPOT));
        }

        public static BigInteger GetTicketPrice()
        {
            return TICKET_PRICE;
        }

        public static bool IsInLockoutPeriod()
        {
            ulong nextDraw = (ulong)(BigInteger)Storage.Get(Storage.CurrentContext,
                PREFIX_CONFIG.Concat(KEY_NEXT_DRAW_TIME));
            return Runtime.Time >= (nextDraw - LOCKOUT_PERIOD);
        }

        public static bool IsPaused()
        {
            return (BigInteger)Storage.Get(Storage.CurrentContext, PREFIX_CONFIG.Concat(KEY_PAUSED)) == 1;
        }

        public static Map<string, object> GetLotteryInfo()
        {
            BigInteger currentDraw = GetCurrentDrawId();
            Draw draw = GetDraw(currentDraw);

            Map<string, object> info = new Map<string, object>();
            info["currentDrawId"] = currentDraw;
            info["nextDrawTime"] = draw.DrawTime;
            info["currentPool"] = draw.TotalPool;
            info["ticketCount"] = draw.TicketCount;
            info["jackpot"] = GetJackpot();
            info["ticketPrice"] = TICKET_PRICE;
            info["isLocked"] = IsInLockoutPeriod();
            info["isPaused"] = IsPaused();

            return info;
        }

        public static Draw[] GetRecentDraws(int count)
        {
            BigInteger currentDraw = GetCurrentDrawId();
            int actualCount = count > (int)currentDraw ? (int)currentDraw : count;

            Draw[] draws = new Draw[actualCount];
            for (int i = 0; i < actualCount; i++)
            {
                draws[i] = GetDraw(currentDraw - i);
            }

            return draws;
        }

        // =====================================================================
        // Internal Functions
        // =====================================================================

        private static void InitializeDraw(BigInteger drawId, ulong drawTime)
        {
            BigInteger carryoverJackpot = GetJackpot();

            Draw draw = new Draw
            {
                DrawId = drawId,
                StartTime = Runtime.Time,
                DrawTime = drawTime,
                WinningNumbers = new byte[0],
                WinningMega = 0,
                TotalPool = carryoverJackpot,
                TicketCount = 0,
                Completed = false,
                VrfRequestId = null,
                WinnerCounts = new BigInteger[5],
                PrizeAmounts = new BigInteger[5]
            };

            ByteString serialized = StdLib.Serialize(draw);
            Storage.Put(Storage.CurrentContext, PREFIX_DRAW.Concat(drawId.ToByteArray()), serialized);
        }

        private static void CompleteDraw(BigInteger drawId, byte[] winningNumbers, byte winningMega)
        {
            Draw draw = GetDraw(drawId);
            draw.WinningNumbers = winningNumbers;
            draw.WinningMega = winningMega;
            draw.Completed = true;

            // Calculate prize distribution
            BigInteger totalPool = draw.TotalPool;

            // Calculate tier pool allocations
            BigInteger[] tierPools = new BigInteger[5];
            tierPools[0] = totalPool * JACKPOT_PERCENT / 100;   // Tier 1: Jackpot
            tierPools[1] = totalPool * SECOND_PERCENT / 100;    // Tier 2
            tierPools[2] = totalPool * THIRD_PERCENT / 100;     // Tier 3
            tierPools[3] = totalPool * FOURTH_PERCENT / 100;    // Tier 4
            tierPools[4] = totalPool * FOURTH_PERCENT / 100;    // Tier 5

            // Count winners per tier by iterating all tickets for this draw
            draw.WinnerCounts = new BigInteger[5];
            BigInteger totalTickets = (BigInteger)Storage.Get(Storage.CurrentContext,
                PREFIX_CONFIG.Concat(KEY_TOTAL_TICKETS));

            for (BigInteger ticketId = 1; ticketId <= totalTickets; ticketId++)
            {
                Ticket ticket = GetTicket(ticketId);
                if (ticket.DrawId != drawId) continue;

                byte tier = CalculatePrizeTier(ticket.MainNumbers, ticket.MegaNumber,
                    winningNumbers, winningMega);

                if (tier > 0 && tier <= 5)
                {
                    draw.WinnerCounts[tier - 1] += 1;
                }
            }

            // Calculate prize per winner for each tier
            draw.PrizeAmounts = new BigInteger[5];
            BigInteger rolloverAmount = 0;

            for (int tier = 0; tier < 5; tier++)
            {
                if (draw.WinnerCounts[tier] > 0)
                {
                    // Divide tier pool among winners
                    draw.PrizeAmounts[tier] = tierPools[tier] / draw.WinnerCounts[tier];
                }
                else
                {
                    // No winners in this tier - add to rollover
                    rolloverAmount += tierPools[tier];
                    draw.PrizeAmounts[tier] = 0;
                }
            }

            // Save completed draw
            ByteString serialized = StdLib.Serialize(draw);
            Storage.Put(Storage.CurrentContext, PREFIX_DRAW.Concat(drawId.ToByteArray()), serialized);

            // Start next draw
            BigInteger nextDrawId = drawId + 1;
            ulong nextDrawTime = (ulong)(draw.DrawTime + DRAW_INTERVAL);

            Storage.Put(Storage.CurrentContext, PREFIX_CONFIG.Concat(KEY_CURRENT_DRAW), nextDrawId);
            Storage.Put(Storage.CurrentContext, PREFIX_CONFIG.Concat(KEY_NEXT_DRAW_TIME), nextDrawTime);

            // Rollover unclaimed funds to jackpot for next draw
            BigInteger currentJackpot = GetJackpot();
            Storage.Put(Storage.CurrentContext, PREFIX_CONFIG.Concat(KEY_JACKPOT), currentJackpot + rolloverAmount);

            if (rolloverAmount > 0)
            {
                OnJackpotRollover(drawId, rolloverAmount);
            }

            InitializeDraw(nextDrawId, nextDrawTime);

            OnDrawCompleted(drawId, winningNumbers, winningMega);
        }

        private static void SaveTicket(Ticket ticket)
        {
            ByteString serialized = StdLib.Serialize(ticket);
            Storage.Put(Storage.CurrentContext, PREFIX_TICKET.Concat(ticket.TicketId.ToByteArray()), serialized);
        }

        private static void AddUserTicket(UInt160 user, BigInteger ticketId)
        {
            BigInteger[] existing = GetUserTickets(user);
            BigInteger[] updated = new BigInteger[existing.Length + 1];

            for (int i = 0; i < existing.Length; i++)
            {
                updated[i] = existing[i];
            }
            updated[existing.Length] = ticketId;

            ByteString serialized = StdLib.Serialize(updated);
            Storage.Put(Storage.CurrentContext, PREFIX_USER_TICKETS.Concat(user), serialized);
        }

        private static void UpdateDrawPool(BigInteger drawId, BigInteger amount)
        {
            Draw draw = GetDraw(drawId);
            draw.TotalPool += amount;
            draw.TicketCount += 1;

            ByteString serialized = StdLib.Serialize(draw);
            Storage.Put(Storage.CurrentContext, PREFIX_DRAW.Concat(drawId.ToByteArray()), serialized);
        }

        private static BigInteger GetAndIncrementTicketCounter()
        {
            BigInteger current = (BigInteger)Storage.Get(Storage.CurrentContext,
                PREFIX_CONFIG.Concat(KEY_TOTAL_TICKETS));
            BigInteger next = current + 1;
            Storage.Put(Storage.CurrentContext, PREFIX_CONFIG.Concat(KEY_TOTAL_TICKETS), next);
            return next;
        }

        private static byte CalculatePrizeTier(byte[] ticketNumbers, byte ticketMega,
            byte[] winningNumbers, byte winningMega)
        {
            // Count matching main numbers
            int mainMatches = 0;
            for (int i = 0; i < MAIN_NUMBERS_COUNT; i++)
            {
                for (int j = 0; j < MAIN_NUMBERS_COUNT; j++)
                {
                    if (ticketNumbers[i] == winningNumbers[j])
                    {
                        mainMatches++;
                        break;
                    }
                }
            }

            bool megaMatch = ticketMega == winningMega;

            // Prize tiers:
            // Tier 1: 5 + Mega (Jackpot)
            // Tier 2: 5 + 0
            // Tier 3: 4 + Mega
            // Tier 4: 4 + 0 or 3 + Mega
            // Tier 5: 3 + 0 or 2 + Mega or 1 + Mega or 0 + Mega

            if (mainMatches == 5 && megaMatch) return 1;
            if (mainMatches == 5) return 2;
            if (mainMatches == 4 && megaMatch) return 3;
            if (mainMatches == 4 || (mainMatches == 3 && megaMatch)) return 4;
            if (mainMatches == 3 || (mainMatches >= 0 && megaMatch)) return 5;

            return 0; // No prize
        }

        private static byte[] SortNumbers(byte[] numbers)
        {
            byte[] sorted = new byte[numbers.Length];
            for (int i = 0; i < numbers.Length; i++)
            {
                sorted[i] = numbers[i];
            }

            // Simple bubble sort
            for (int i = 0; i < sorted.Length - 1; i++)
            {
                for (int j = 0; j < sorted.Length - i - 1; j++)
                {
                    if (sorted[j] > sorted[j + 1])
                    {
                        byte temp = sorted[j];
                        sorted[j] = sorted[j + 1];
                        sorted[j + 1] = temp;
                    }
                }
            }

            return sorted;
        }

        private static bool ContainsNumber(byte[] numbers, byte num, int length)
        {
            for (int i = 0; i < length; i++)
            {
                if (numbers[i] == num) return true;
            }
            return false;
        }

        private static UInt160 GetVRFContract()
        {
            return (UInt160)Storage.Get(Storage.CurrentContext, PREFIX_CONFIG.Concat(KEY_VRF_CONTRACT));
        }

        private static UInt160 GetAutomationContract()
        {
            return (UInt160)Storage.Get(Storage.CurrentContext, PREFIX_CONFIG.Concat(KEY_AUTOMATION_CONTRACT));
        }

        private static bool IsAdmin()
        {
            UInt160 admin = (UInt160)Storage.Get(Storage.CurrentContext, PREFIX_ADMIN);
            return Runtime.CheckWitness(admin);
        }

        private static void ValidateAdmin()
        {
            ExecutionEngine.Assert(IsAdmin(), "Admin only");
        }
    }
}
