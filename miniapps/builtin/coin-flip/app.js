/**
 * Coin Flip MiniApp
 * Simple 50/50 game with on-chain randomness for Neo N3
 * @version 1.0.0
 */

"use strict";

const CONFIG = {
  APP_ID: "builtin-coin-flip",
  GAS_DECIMALS: 8,
  MIN_BET: 0.1,
  MAX_BET: 10,
  MULTIPLIER: 2,
};

let sdk = null;
let selectedChoice = "heads";
let isFlipping = false;
let stats = { wins: 0, losses: 0, streak: 0, currentStreak: 0 };
let flipHistory = [];

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

async function requestRandomValue(maxExclusive) {
  if (!sdk || !sdk.rng || typeof sdk.rng.requestRandom !== "function") {
    throw new Error("Randomness API not available");
  }
  const res = await sdk.rng.requestRandom(CONFIG.APP_ID);
  const bytes = hexToBytes(res?.randomness ?? "");
  return randomIntFromBytes(bytes, 0, maxExclusive);
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
 * Set button loading state
 */
function setButtonLoading(loading) {
  const btn = document.getElementById("btn-flip");
  const coin = document.getElementById("coin");
  isFlipping = loading;

  if (loading) {
    btn.disabled = true;
    btn.textContent = "Flipping...";
    btn.setAttribute("aria-busy", "true");
    coin.classList.add("flipping");
  } else {
    btn.disabled = false;
    btn.textContent = "Flip Coin";
    btn.removeAttribute("aria-busy");
    coin.classList.remove("flipping");
  }
}

/**
 * Select choice (heads or tails)
 */
function selectChoice(choice) {
  selectedChoice = choice;
  document.querySelectorAll(".choice-btn").forEach((btn) => {
    const isSelected = btn.dataset.choice === choice;
    btn.classList.toggle("selected", isSelected);
    btn.setAttribute("aria-checked", isSelected ? "true" : "false");
  });
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
  const potentialWin = amount * CONFIG.MULTIPLIER;
  document.getElementById("potential-win").textContent = `${formatNumber(potentialWin)} GAS`;
}

/**
 * Update stats display
 */
function updateStats() {
  document.getElementById("wins").textContent = stats.wins;
  document.getElementById("losses").textContent = stats.losses;
  document.getElementById("streak").textContent =
    stats.currentStreak > 0 ? `+${stats.currentStreak}` : stats.currentStreak;
}

/**
 * Flip coin with on-chain randomness
 */
async function flipCoin() {
  if (isFlipping) return;

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
    showStatus("Flipping coin...", "loading");

    let result;

    await submitPayment(amountText, `coin-flip:${selectedChoice}`);
    const randomValue = await requestRandomValue(2);
    result = randomValue === 0 ? "heads" : "tails";

    const coin = document.getElementById("coin");
    coin.textContent = result === "heads" ? "H" : "T";
    coin.className = `coin ${result}`;
    coin.setAttribute("aria-label", `Coin showing ${result}`);

    const won = result === selectedChoice;
    const payout = won ? amount * CONFIG.MULTIPLIER : 0;

    if (won) {
      stats.wins++;
      stats.currentStreak = stats.currentStreak >= 0 ? stats.currentStreak + 1 : 1;
      if (stats.currentStreak > stats.streak) stats.streak = stats.currentStreak;
      showStatus(`You won ${formatNumber(payout)} GAS!`, "success");
    } else {
      stats.losses++;
      stats.currentStreak = stats.currentStreak <= 0 ? stats.currentStreak - 1 : -1;
      showStatus(`You lost ${formatNumber(amount)} GAS. The coin landed on ${result}.`, "error");
    }

    updateStats();
    addToHistory(result, selectedChoice, amount, won, payout);
  } catch (err) {
    const errorMsg = err.message || String(err);
    if (errorMsg.includes("insufficient")) {
      showStatus("Insufficient GAS balance", "error");
    } else if (errorMsg.includes("rejected") || errorMsg.includes("cancelled")) {
      showStatus("Transaction was cancelled", "error");
    } else {
      showStatus(`Error: ${sanitize(errorMsg)}`, "error");
    }
    console.error("Flip coin error:", err);
  } finally {
    setButtonLoading(false);
  }
}

/**
 * Add flip to history
 */
function addToHistory(result, choice, amount, won, payout) {
  flipHistory.unshift({
    result,
    choice,
    amount,
    won,
    payout,
    time: new Date(),
  });

  if (flipHistory.length > 20) flipHistory.pop();

  renderHistory();
  saveData();
}

/**
 * Render history list (XSS-safe)
 */
function renderHistory() {
  const list = document.getElementById("history-list");
  list.innerHTML = "";

  if (flipHistory.length === 0) {
    const item = document.createElement("div");
    item.className = "history-item";
    item.setAttribute("role", "listitem");
    item.textContent = "No flips yet";
    list.appendChild(item);
    return;
  }

  flipHistory.forEach((flip) => {
    const item = document.createElement("div");
    item.className = `history-item ${flip.won ? "win" : "lose"}`;
    item.setAttribute("role", "listitem");

    const resultSpan = document.createElement("span");
    resultSpan.textContent = `${flip.result.toUpperCase()} (bet: ${flip.choice})`;

    const amountSpan = document.createElement("span");
    amountSpan.className = flip.won ? "result-win" : "result-lose";
    amountSpan.textContent = flip.won ? `+${formatNumber(flip.payout)}` : `-${formatNumber(flip.amount)}`;

    item.appendChild(resultSpan);
    item.appendChild(amountSpan);
    list.appendChild(item);
  });
}

/**
 * Save data to storage
 */
function saveData() {
  try {
    const data = JSON.stringify({ stats, history: flipHistory });
    localStorage.setItem("coinflip_data", data);
  } catch (err) {
    console.warn("Could not save data:", err);
  }
}

/**
 * Load data from storage
 */
function loadData() {
  try {
    const data = localStorage.getItem("coinflip_data");

    if (data) {
      const parsed = JSON.parse(data);
      stats = parsed.stats || stats;
      flipHistory = (parsed.history || []).map((f) => ({ ...f, time: new Date(f.time) }));
    }
  } catch (err) {
    console.warn("Could not load data:", err);
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

  loadData();
  updateStats();
  renderHistory();
  updatePotentialWin();

  input.addEventListener("input", updatePotentialWin);
  input.addEventListener("change", updatePotentialWin);

  input.addEventListener("keypress", (e) => {
    if (e.key === "Enter" && !isFlipping) {
      flipCoin();
    }
  });

  console.log("Coin Flip MiniApp initialized");
}

if (document.readyState === "loading") {
  document.addEventListener("DOMContentLoaded", init);
} else {
  init();
}
