/**
 * Dice Game MiniApp
 * Provably fair dice game with on-chain randomness for Neo N3
 * @version 1.0.0
 */

"use strict";

const CONFIG = {
  APP_ID: "builtin-dice-game",
  GAS_DECIMALS: 8,
  MIN_BET: 0.1,
  MAX_BET: 10,
  MULTIPLIERS: {
    over7: 2,
    under7: 2,
    exact7: 6,
    doubles: 6,
    even: 2,
    odd: 2,
  },
};

let sdk = null;
let selectedBet = "over7";
let isRolling = false;
let rollHistory = [];

/**
 * Sanitize string to prevent XSS
 */
function sanitize(str) {
  const div = document.createElement("div");
  div.textContent = str;
  return div.innerHTML;
}

/**
 * Format number with decimals
 */
function formatNumber(num, decimals = 2) {
  return Number(num).toLocaleString(undefined, {
    minimumFractionDigits: decimals,
    maximumFractionDigits: decimals,
  });
}

function formatAmountInput(value) {
  if (typeof value === "number" && Number.isFinite(value)) {
    return value.toFixed(CONFIG.GAS_DECIMALS).replace(/\.?0+$/, "");
  }
  const trimmed = String(value ?? "").trim();
  if (!trimmed) return "0";
  return trimmed;
}

function hexToBytes(hex) {
  const cleaned = String(hex ?? "").trim().replace(/^0x/i, "");
  if (!cleaned || cleaned.length % 2 !== 0) return [];
  const bytes = [];
  for (let i = 0; i < cleaned.length; i += 2) {
    const byte = Number.parseInt(cleaned.slice(i, i + 2), 16);
    if (Number.isNaN(byte)) return [];
    bytes.push(byte);
  }
  return bytes;
}

function randomIntFromBytes(bytes, offset, maxExclusive) {
  if (!bytes.length) {
    throw new Error("Randomness bytes missing");
  }
  const high = bytes[offset % bytes.length];
  const low = bytes[(offset + 1) % bytes.length];
  const combined = (high << 8) | low;
  return combined % maxExclusive;
}

async function requestRandomDice() {
  if (!sdk || !sdk.rng || typeof sdk.rng.requestRandom !== "function") {
    throw new Error("Randomness API not available");
  }
  const res = await sdk.rng.requestRandom(CONFIG.APP_ID);
  const bytes = hexToBytes(res?.randomness ?? "");
  const first = randomIntFromBytes(bytes, 0, 6) + 1;
  const second = randomIntFromBytes(bytes, 2, 6) + 1;
  return [first, second];
}

async function submitPayment(amountText, memo) {
  if (!sdk || !sdk.payments || typeof sdk.payments.payGAS !== "function") {
    throw new Error("Payment API not available");
  }
  const intent = await sdk.payments.payGAS(CONFIG.APP_ID, amountText, memo);
  if (sdk.wallet && typeof sdk.wallet.invokeIntent === "function") {
    await sdk.wallet.invokeIntent(intent.request_id);
  } else {
    showStatus("Payment intent created. Approve in host.", "loading");
  }
  return true;
}

/**
 * Show status message
 */
function showStatus(message, type) {
  const status = document.getElementById("status");
  status.textContent = sanitize(message);
  status.className = `status ${type}`;
  status.setAttribute("role", "alert");
}

/**
 * Clear status message
 */
function clearStatus() {
  const status = document.getElementById("status");
  status.textContent = "";
  status.className = "status";
  status.removeAttribute("role");
}

/**
 * Set button loading state
 */
function setButtonLoading(loading) {
  const btn = document.getElementById("btn-roll");
  isRolling = loading;

  if (loading) {
    btn.disabled = true;
    btn.textContent = "Rolling...";
    btn.setAttribute("aria-busy", "true");
    document.getElementById("dice1").classList.add("rolling");
    document.getElementById("dice2").classList.add("rolling");
  } else {
    btn.disabled = false;
    btn.textContent = "Roll Dice";
    btn.removeAttribute("aria-busy");
    document.getElementById("dice1").classList.remove("rolling");
    document.getElementById("dice2").classList.remove("rolling");
  }
}

/**
 * Select bet type
 */
function selectBet(betType) {
  selectedBet = betType;

  document.querySelectorAll(".bet-type-btn").forEach((btn) => {
    const isSelected = btn.dataset.bet === betType;
    btn.classList.toggle("selected", isSelected);
    btn.setAttribute("aria-checked", isSelected ? "true" : "false");
  });

  updatePotentialWin();
}

/**
 * Set bet amount
 */
function setAmount(amount) {
  document.getElementById("bet-amount").value = amount;
  updatePotentialWin();
}

/**
 * Update potential win display
 */
function updatePotentialWin() {
  const amount = parseFloat(document.getElementById("bet-amount").value) || 0;
  const multiplier = CONFIG.MULTIPLIERS[selectedBet] || 2;
  const potentialWin = amount * multiplier;
  document.getElementById("potential-win").textContent = `${formatNumber(potentialWin)} GAS`;
}

/**
 * Check if bet wins
 */
function checkWin(dice1, dice2, betType) {
  const total = dice1 + dice2;

  switch (betType) {
    case "over7":
      return total > 7;
    case "under7":
      return total < 7;
    case "exact7":
      return total === 7;
    case "doubles":
      return dice1 === dice2;
    case "even":
      return total % 2 === 0;
    case "odd":
      return total % 2 === 1;
    default:
      return false;
  }
}

/**
 * Roll dice with on-chain randomness
 */
async function rollDice() {
  if (isRolling) return;

  const input = document.getElementById("bet-amount");
  const amountText = formatAmountInput(input.value);
  const amount = parseFloat(amountText) || 0;

  if (amount < CONFIG.MIN_BET) {
    showStatus(`Minimum bet is ${CONFIG.MIN_BET} GAS`, "error");
    return;
  }

  if (amount > CONFIG.MAX_BET) {
    showStatus(`Maximum bet is ${CONFIG.MAX_BET} GAS`, "error");
    return;
  }

  sdk = window.MiniAppSDK;
  if (!sdk || !sdk.rng) {
    showStatus("MiniAppSDK RNG not available. Please open in the platform host.", "error");
    return;
  }

  try {
    setButtonLoading(true);
    showStatus("Rolling dice...", "loading");

    let dice1, dice2;

    await submitPayment(amountText, `dice:${selectedBet}`);
    [dice1, dice2] = await requestRandomDice();

    // Update dice display
    document.getElementById("dice1").textContent = dice1;
    document.getElementById("dice2").textContent = dice2;

    const total = dice1 + dice2;
    const won = checkWin(dice1, dice2, selectedBet);
    const multiplier = CONFIG.MULTIPLIERS[selectedBet];
    const payout = won ? amount * multiplier : 0;

    document.getElementById("total-roll").textContent = total;
    document.getElementById("last-result").textContent = won ? "WIN" : "LOSE";
    document.getElementById("last-result").style.color = won ? "#00ff88" : "#ff6464";

    if (won) {
      showStatus(`You won ${formatNumber(payout)} GAS!`, "success");
    } else {
      showStatus(`You lost ${formatNumber(amount)} GAS. Try again!`, "error");
    }

    // Add to history
    addToHistory(dice1, dice2, selectedBet, amount, won, payout);
  } catch (err) {
    const errorMsg = err.message || String(err);
    if (errorMsg.includes("insufficient")) {
      showStatus("Insufficient GAS balance", "error");
    } else if (errorMsg.includes("rejected") || errorMsg.includes("cancelled")) {
      showStatus("Transaction was cancelled", "error");
    } else {
      showStatus(`Error: ${sanitize(errorMsg)}`, "error");
    }
    console.error("Roll dice error:", err);
  } finally {
    setButtonLoading(false);
  }
}

/**
 * Add roll to history
 */
function addToHistory(dice1, dice2, betType, amount, won, payout) {
  rollHistory.unshift({
    dice1,
    dice2,
    total: dice1 + dice2,
    betType,
    amount,
    won,
    payout,
    time: new Date(),
  });

  if (rollHistory.length > 20) {
    rollHistory.pop();
  }

  renderHistory();
  saveHistory();
}

/**
 * Render history list (XSS-safe)
 */
function renderHistory() {
  const list = document.getElementById("history-list");
  list.innerHTML = "";

  if (rollHistory.length === 0) {
    const item = document.createElement("div");
    item.className = "history-item";
    item.setAttribute("role", "listitem");
    item.textContent = "No rolls yet";
    list.appendChild(item);
    return;
  }

  rollHistory.forEach((roll) => {
    const item = document.createElement("div");
    item.className = `history-item ${roll.won ? "win" : "lose"}`;
    item.setAttribute("role", "listitem");

    const diceSpan = document.createElement("span");
    diceSpan.className = "history-dice";
    diceSpan.textContent = `${roll.dice1} + ${roll.dice2} = ${roll.total}`;

    const betSpan = document.createElement("span");
    betSpan.textContent = roll.betType;
    betSpan.style.color = "#888";

    const resultSpan = document.createElement("span");
    resultSpan.className = `history-result ${roll.won ? "win" : "lose"}`;
    resultSpan.textContent = roll.won ? `+${formatNumber(roll.payout)}` : `-${formatNumber(roll.amount)}`;

    item.appendChild(diceSpan);
    item.appendChild(betSpan);
    item.appendChild(resultSpan);
    list.appendChild(item);
  });
}

/**
 * Save history to storage
 */
function saveHistory() {
  try {
    const data = JSON.stringify(rollHistory);
    localStorage.setItem("dice_history", data);
  } catch (err) {
    console.warn("Could not save history:", err);
  }
}

/**
 * Load history from storage
 */
function loadHistory() {
  try {
    const data = localStorage.getItem("dice_history");

    if (data) {
      rollHistory = JSON.parse(data).map((r) => ({
        ...r,
        time: new Date(r.time),
      }));
    }
  } catch (err) {
    console.warn("Could not load history:", err);
    rollHistory = [];
  }
}

/**
 * Initialize the MiniApp
 */
async function init() {
  sdk = window.MiniAppSDK;

  const input = document.getElementById("bet-amount");
  input.min = CONFIG.MIN_BET;
  input.max = CONFIG.MAX_BET;

  loadHistory();
  renderHistory();
  updatePotentialWin();

  input.addEventListener("input", updatePotentialWin);
  input.addEventListener("change", updatePotentialWin);

  input.addEventListener("keypress", (e) => {
    if (e.key === "Enter" && !isRolling) {
      rollDice();
    }
  });

  console.log("Dice Game MiniApp initialized");
}

if (document.readyState === "loading") {
  document.addEventListener("DOMContentLoaded", init);
} else {
  init();
}
