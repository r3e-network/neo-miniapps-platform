/**
 * Lottery MiniApp
 * Decentralized lottery with provably fair randomness for Neo N3
 * @version 1.0.0
 */

"use strict";

const CONFIG = {
  APP_ID: "builtin-lottery",
  GAS_DECIMALS: 8,
  TICKET_PRICE: 0.1,
  MIN_TICKETS: 1,
  MAX_TICKETS: 100,
  ROUND_DURATION: 60000,
  REFRESH_INTERVAL: 10000,
};

let sdk = null;
let userAddress = null;
let currentRound = 1;
let roundStartTime = Date.now();
let prizePool = 0;
let ticketCount = 0;
let userTickets = 0;
let isExecuting = false;
let isDrawing = false;
let recentWinners = [];

/**
 * Sanitize string to prevent XSS
 */
function sanitize(str) {
  const div = document.createElement("div");
  div.textContent = str;
  return div.innerHTML;
}

/**
 * Format number with specified decimals
 */
function formatNumber(num, decimals = 2) {
  return Number(num).toLocaleString(undefined, {
    minimumFractionDigits: decimals,
    maximumFractionDigits: decimals,
  });
}

/**
 * Format GAS amount from raw value
 */
function formatGAS(amount) {
  return formatNumber(Number(amount), 2);
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
  let tx = null;
  if (sdk.wallet && typeof sdk.wallet.invokeIntent === "function") {
    tx = await sdk.wallet.invokeIntent(intent.request_id);
  } else {
    showStatus("Payment intent created. Approve in host.", "loading");
  }
  return { intent, tx };
}

/**
 * Show status message
 */
function showStatus(message, type) {
  const status = document.getElementById("status");
  status.textContent = sanitize(message);
  status.className = `status ${type}`;
  status.setAttribute("role", "alert");

  if (type === "success") {
    setTimeout(clearStatus, 5000);
  }
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
  const btn = document.getElementById("btn-buy");
  isExecuting = loading;

  if (loading) {
    btn.disabled = true;
    btn.textContent = "Processing...";
    btn.setAttribute("aria-busy", "true");
  } else {
    btn.disabled = false;
    btn.textContent = "Buy Tickets";
    btn.removeAttribute("aria-busy");
  }
}

/**
 * Adjust ticket count
 */
function adjustTickets(delta) {
  const input = document.getElementById("ticket-amount");
  let value = parseInt(input.value) || 1;
  value = Math.max(CONFIG.MIN_TICKETS, Math.min(CONFIG.MAX_TICKETS, value + delta));
  input.value = value;
  updateTotalCost();
}

/**
 * Update total cost display
 */
function updateTotalCost() {
  const input = document.getElementById("ticket-amount");
  const amount = parseInt(input.value) || 1;

  if (amount < CONFIG.MIN_TICKETS) {
    input.setCustomValidity(`Minimum is ${CONFIG.MIN_TICKETS} ticket`);
  } else if (amount > CONFIG.MAX_TICKETS) {
    input.setCustomValidity(`Maximum is ${CONFIG.MAX_TICKETS} tickets`);
  } else {
    input.setCustomValidity("");
  }

  const total = amount * CONFIG.TICKET_PRICE;
  document.getElementById("total-cost").textContent = `${formatNumber(total, 1)} GAS`;
}

/**
 * Update countdown timer
 */
function updateCountdown() {
  const now = Date.now();
  const elapsed = now - roundStartTime;
  const remaining = Math.max(0, CONFIG.ROUND_DURATION - (elapsed % CONFIG.ROUND_DURATION));
  const seconds = Math.floor(remaining / 1000);
  document.getElementById("countdown").textContent = `Next draw in: 0:${seconds.toString().padStart(2, "0")}`;

  if (remaining === 0) {
    void completeRound();
  }
}

async function completeRound() {
  if (isDrawing) return;
  isDrawing = true;

  try {
    if (ticketCount > 0) {
      const winnerIndex = await requestRandomValue(ticketCount);
      let winnerAddress = "ANON-WINNER";
      if (userTickets > 0 && userAddress && winnerIndex < userTickets) {
        winnerAddress = userAddress;
      }

      recentWinners.unshift({
        round: currentRound,
        address: winnerAddress,
        prize: prizePool,
      });
      if (recentWinners.length > 10) recentWinners.pop();
    }

    currentRound += 1;
    roundStartTime = Date.now();
    prizePool = 0;
    ticketCount = 0;
    userTickets = 0;
    renderRoundInfo();
    renderWinners();
  } finally {
    isDrawing = false;
  }
}

/**
 * Refresh round information from contract
 */
async function refreshRoundInfo() {
  try {
    renderRoundInfo();
  } catch (err) {
    console.error("Error refreshing round info:", err);
  }
}

/**
 * Render round info
 */
function renderRoundInfo() {
  document.getElementById("round-id").textContent = currentRound;
  document.getElementById("prize-pool").textContent = formatNumber(prizePool);
  document.getElementById("ticket-count").textContent = ticketCount;
  document.getElementById("your-tickets").textContent = userTickets;
}

/**
 * Load recent winners (XSS-safe)
 */
async function loadWinners() {
  const winnersList = document.getElementById("winners-list");

  try {
    renderWinners();
  } catch (err) {
    console.error("Error loading winners:", err);
    renderWinners();
  }
}

/**
 * Render winners list (XSS-safe DOM manipulation)
 */
function renderWinners() {
  const list = document.getElementById("winners-list");
  list.innerHTML = "";

  if (recentWinners.length === 0) {
    const item = document.createElement("div");
    item.className = "winner-item";
    item.setAttribute("role", "listitem");

    const span = document.createElement("span");
    span.className = "winner-address";
    span.textContent = "No winners yet";
    item.appendChild(span);
    list.appendChild(item);
    return;
  }

  recentWinners.forEach((winner) => {
    const item = document.createElement("div");
    item.className = "winner-item";
    item.setAttribute("role", "listitem");

    const roundSpan = document.createElement("span");
    roundSpan.className = "winner-round";
    roundSpan.textContent = `Round #${winner.round}`;

    const addrSpan = document.createElement("span");
    addrSpan.className = "winner-address";
    const shortAddr = winner.address.substring(0, 8) + "..." + winner.address.substring(winner.address.length - 6);
    addrSpan.textContent = shortAddr;
    addrSpan.title = winner.address;

    const prizeSpan = document.createElement("span");
    prizeSpan.className = "winner-prize";
    prizeSpan.textContent = `${formatGAS(winner.prize)} GAS`;

    item.appendChild(roundSpan);
    item.appendChild(addrSpan);
    item.appendChild(prizeSpan);
    list.appendChild(item);
  });
}

/**
 * Buy lottery tickets
 */
async function buyTickets() {
  if (isExecuting) return;

  const input = document.getElementById("ticket-amount");
  const amount = parseInt(input.value) || 1;

  if (!input.checkValidity()) {
    showStatus(input.validationMessage || "Invalid amount", "error");
    return;
  }

  if (amount < CONFIG.MIN_TICKETS) {
    showStatus(`Minimum is ${CONFIG.MIN_TICKETS} ticket`, "error");
    return;
  }

  if (amount > CONFIG.MAX_TICKETS) {
    showStatus(`Maximum is ${CONFIG.MAX_TICKETS} tickets`, "error");
    return;
  }

  sdk = window.MiniAppSDK;
  if (!sdk) {
    showStatus("MiniAppSDK not available. Please open in the platform host.", "error");
    return;
  }

  if (!sdk.wallet || typeof sdk.wallet.getAddress !== "function") {
    showStatus("Please connect your wallet first", "error");
    return;
  }

  try {
    setButtonLoading(true);
    showStatus(`Purchasing ${amount} ticket(s)...`, "loading");

    if (!userAddress && sdk.wallet) {
      const addrResult = await sdk.wallet.getAddress();
      const addr = String(addrResult ?? "").trim();
      if (addr) {
        userAddress = addr;
      }
    }

    const totalGAS = amount * CONFIG.TICKET_PRICE;
    const amountText = formatAmountInput(totalGAS);

    const payment = await submitPayment(amountText, `lottery:${currentRound}:${amount}`);

    showStatus(`Success! Purchased ${amount} ticket(s).`, "success");
    saveTicketPurchase(payment.intent.request_id, amount, totalGAS);

    ticketCount += amount;
    userTickets += amount;
    prizePool += totalGAS;
    renderRoundInfo();
  } catch (err) {
    const errorMsg = err.message || String(err);
    if (errorMsg.includes("insufficient")) {
      showStatus("Insufficient GAS balance", "error");
    } else if (errorMsg.includes("rejected") || errorMsg.includes("cancelled")) {
      showStatus("Transaction was cancelled", "error");
    } else {
      showStatus(`Error: ${sanitize(errorMsg)}`, "error");
    }
    console.error("Buy tickets error:", err);
  } finally {
    setButtonLoading(false);
  }
}

/**
 * Save ticket purchase to storage
 */
function saveTicketPurchase(referenceId, amount, cost) {
  try {
    const purchase = {
      reference_id: referenceId,
      round: currentRound,
      amount: amount,
      cost: cost,
      time: new Date().toISOString(),
    };

    let history = [];
    const data = localStorage.getItem("lottery_history");
    if (data) history = JSON.parse(data);

    history.unshift(purchase);
    if (history.length > 50) history = history.slice(0, 50);

    localStorage.setItem("lottery_history", JSON.stringify(history));
  } catch (err) {
    console.warn("Could not save purchase history:", err);
  }
}

/**
 * Initialize the MiniApp
 */
async function init() {
  const input = document.getElementById("ticket-amount");
  input.min = CONFIG.MIN_TICKETS;
  input.max = CONFIG.MAX_TICKETS;

  sdk = window.MiniAppSDK;

  if (sdk && sdk.wallet && typeof sdk.wallet.getAddress === "function") {
    try {
      const addrResult = await sdk.wallet.getAddress();
      const addr = String(addrResult ?? "").trim();
      if (addr) {
        userAddress = addr;
      }
    } catch (e) {
      console.log("Could not get wallet address:", e);
    }
  }

  updateTotalCost();
  await refreshRoundInfo();
  await loadWinners();

  setInterval(updateCountdown, 1000);
  setInterval(async () => {
    await refreshRoundInfo();
    await loadWinners();
  }, CONFIG.REFRESH_INTERVAL);

  input.addEventListener("input", updateTotalCost);
  input.addEventListener("change", updateTotalCost);

  input.addEventListener("keypress", (e) => {
    if (e.key === "Enter" && !isExecuting) {
      buyTickets();
    }
  });

  console.log("Lottery MiniApp initialized");
}

if (document.readyState === "loading") {
  document.addEventListener("DOMContentLoaded", init);
} else {
  init();
}
