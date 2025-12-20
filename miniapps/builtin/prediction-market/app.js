/**
 * Prediction Market MiniApp
 * Decentralized price prediction market for Neo N3
 * @version 1.0.0
 */

"use strict";

const CONFIG = {
  APP_ID: "builtin-prediction-market",
  GAS_DECIMALS: 8,
  MIN_BET: 0.1,
  MAX_BET: 100,
  ROUND_DURATION: 300000, // 5 minutes
  REFRESH_INTERVAL: 5000,
};

let sdk = null;
let selectedDirection = "up";
let isPlacing = false;
let currentRound = 1;
let roundStartTime = Date.now();
let currentPrice = 12.5;
let lockPrice = 12.5;
let upPool = 0;
let downPool = 0;
let predictions = [];

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
  const trimmed = String(value ?? "").trim();
  if (!trimmed) return "0";
  return trimmed;
}

async function submitPayment(amountText, memo) {
  if (!sdk || !sdk.payments || typeof sdk.payments.payGAS !== "function") {
    return null;
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
}

/**
 * Set button loading state
 */
function setButtonLoading(loading) {
  const btn = document.getElementById("btn-predict");
  isPlacing = loading;

  if (loading) {
    btn.disabled = true;
    btn.textContent = "Placing...";
    btn.setAttribute("aria-busy", "true");
  } else {
    btn.disabled = false;
    btn.textContent = "Place Prediction";
    btn.removeAttribute("aria-busy");
  }
}

/**
 * Select prediction direction
 */
function selectDirection(direction) {
  selectedDirection = direction;
  document.querySelectorAll(".pred-btn").forEach((btn) => {
    const isSelected = btn.dataset.direction === direction;
    btn.classList.toggle("selected", isSelected);
    btn.setAttribute("aria-checked", isSelected ? "true" : "false");
  });
  updatePotentialPayout();
}

/**
 * Set bet amount
 */
function setAmount(amount) {
  document.getElementById("bet-amount").value = amount;
  updatePotentialPayout();
}

/**
 * Update potential payout display
 */
function updatePotentialPayout() {
  const amount = parseFloat(document.getElementById("bet-amount").value) || 0;
  const totalPool = upPool + downPool + amount;
  const myPool = selectedDirection === "up" ? upPool + amount : downPool + amount;
  const otherPool = selectedDirection === "up" ? downPool : upPool;

  let payout = 1;
  if (myPool > 0) {
    payout = (totalPool / myPool).toFixed(2);
  }

  document.getElementById("potential-payout").textContent = `${payout}x`;
}

/**
 * Update countdown timer
 */
function updateCountdown() {
  const now = Date.now();
  const elapsed = now - roundStartTime;
  const remaining = Math.max(0, CONFIG.ROUND_DURATION - elapsed);
  const minutes = Math.floor(remaining / 60000);
  const seconds = Math.floor((remaining % 60000) / 1000);
  document.getElementById("countdown").textContent = `${minutes}:${seconds.toString().padStart(2, "0")}`;

  if (remaining <= 0) {
    resolveRound();
  }
}

/**
 * Fetch current price from datafeed
 */
async function fetchPrice() {
  sdk = window.MiniAppSDK;

  try {
    if (!sdk || !sdk.datafeed || typeof sdk.datafeed.getPrice !== "function") {
      showStatus("MiniAppSDK datafeed not available. Please open in the platform host.", "error");
      return;
    }
    const priceResult = await sdk.datafeed.getPrice("NEO-USD");
    if (priceResult && priceResult.price) {
      const newPrice = parseFloat(priceResult.price);
      updatePriceDisplay(newPrice);
    }
  } catch (err) {
    console.error("Error fetching price:", err);
  }
}

/**
 * Update price display
 */
function updatePriceDisplay(newPrice) {
  const priceChange = ((newPrice - lockPrice) / lockPrice) * 100;
  currentPrice = newPrice;

  document.getElementById("current-price").textContent = `$${formatNumber(currentPrice)}`;

  const changeEl = document.getElementById("price-change");
  changeEl.textContent = `${priceChange >= 0 ? "+" : ""}${formatNumber(priceChange)}%`;
  changeEl.className = `price-change ${priceChange >= 0 ? "up" : "down"}`;
}

/**
 * Refresh round info from contract
 */
async function refreshRoundInfo() {
  document.getElementById("round-id").textContent = `#${currentRound}`;
  document.getElementById("lock-price").textContent = `$${formatNumber(lockPrice)}`;
  document.getElementById("up-pool").textContent = `${formatNumber(upPool)} GAS`;
  document.getElementById("down-pool").textContent = `${formatNumber(downPool)} GAS`;

  updatePotentialPayout();
}

/**
 * Place a prediction
 */
async function placePrediction() {
  if (isPlacing) return;

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

  try {
    setButtonLoading(true);
    showStatus(`Placing ${selectedDirection.toUpperCase()} prediction...`, "loading");

    if (!sdk || !sdk.payments || typeof sdk.payments.payGAS !== "function") {
      showStatus("MiniAppSDK payments not available. Please open in the platform host.", "error");
      return;
    }

    const payment = await submitPayment(amountText, `prediction:${currentRound}:${selectedDirection}`);
    if (!payment) {
      showStatus("Payment API not available", "error");
      return;
    }
    showStatus("Prediction placed successfully!", "success");

    // Update pools locally
    if (selectedDirection === "up") {
      upPool += amount;
    } else {
      downPool += amount;
    }

    // Add to predictions
    addPrediction(currentRound, selectedDirection, amount, lockPrice);

    refreshRoundInfo();
  } catch (err) {
    const errorMsg = err.message || String(err);
    if (errorMsg.includes("insufficient")) {
      showStatus("Insufficient GAS balance", "error");
    } else if (errorMsg.includes("rejected") || errorMsg.includes("cancelled")) {
      showStatus("Transaction was cancelled", "error");
    } else {
      showStatus(`Error: ${sanitize(errorMsg)}`, "error");
    }
    console.error("Place prediction error:", err);
  } finally {
    setButtonLoading(false);
  }
}

/**
 * Add prediction to history
 */
function addPrediction(round, direction, amount, lockPriceAtBet) {
  predictions.unshift({
    round,
    direction,
    amount,
    lockPrice: lockPriceAtBet,
    status: "pending",
    payout: 0,
    time: new Date(),
  });

  if (predictions.length > 20) predictions.pop();

  renderHistory();
  saveData();
}

/**
 * Resolve current round
 */
function resolveRound() {
  // Check pending predictions for this round
  predictions.forEach((pred) => {
    if (pred.round === currentRound && pred.status === "pending") {
      const won =
        (pred.direction === "up" && currentPrice > pred.lockPrice) ||
        (pred.direction === "down" && currentPrice < pred.lockPrice);

      if (won) {
        const totalPool = upPool + downPool;
        const winningPool = pred.direction === "up" ? upPool : downPool;
        const multiplier = winningPool > 0 ? totalPool / winningPool : 1;
        pred.payout = pred.amount * multiplier;
        pred.status = "win";
      } else {
        pred.status = "lose";
      }
    }
  });

  // Start new round
  currentRound++;
  roundStartTime = Date.now();
  lockPrice = currentPrice;
  upPool = 0;
  downPool = 0;

  renderHistory();
  refreshRoundInfo();
  saveData();
}

/**
 * Render history list (XSS-safe)
 */
function renderHistory() {
  const list = document.getElementById("history-list");
  list.innerHTML = "";

  if (predictions.length === 0) {
    const item = document.createElement("div");
    item.className = "history-item";
    item.setAttribute("role", "listitem");
    item.textContent = "No predictions yet";
    list.appendChild(item);
    return;
  }

  predictions.forEach((pred) => {
    const item = document.createElement("div");
    item.className = `history-item ${pred.status}`;
    item.setAttribute("role", "listitem");

    const roundSpan = document.createElement("span");
    roundSpan.textContent = `Round #${pred.round}`;
    roundSpan.style.color = "#888";

    const predSpan = document.createElement("span");
    predSpan.className = `history-pred ${pred.direction}`;
    predSpan.textContent = `${pred.direction.toUpperCase()} ${formatNumber(pred.amount)} GAS`;

    const resultSpan = document.createElement("span");
    resultSpan.className = `history-result ${pred.status}`;
    if (pred.status === "pending") {
      resultSpan.textContent = "Pending";
    } else if (pred.status === "win") {
      resultSpan.textContent = `+${formatNumber(pred.payout)} GAS`;
    } else {
      resultSpan.textContent = `-${formatNumber(pred.amount)} GAS`;
    }

    item.appendChild(roundSpan);
    item.appendChild(predSpan);
    item.appendChild(resultSpan);
    list.appendChild(item);
  });
}

/**
 * Save data to storage
 */
function saveData() {
  try {
    const data = JSON.stringify({
      currentRound,
      predictions,
    });
    localStorage.setItem("prediction_data", data);
  } catch (err) {
    console.warn("Could not save data:", err);
  }
}

/**
 * Load data from storage
 */
function loadData() {
  try {
    const data = localStorage.getItem("prediction_data");

    if (data) {
      const parsed = JSON.parse(data);
      currentRound = parsed.currentRound || 1;
      predictions = (parsed.predictions || []).map((p) => ({
        ...p,
        time: new Date(p.time),
      }));
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
  renderHistory();

  await fetchPrice();
  await refreshRoundInfo();

  // Update countdown every second
  setInterval(updateCountdown, 1000);

  // Refresh price and round info periodically
  setInterval(async () => {
    await fetchPrice();
    await refreshRoundInfo();
  }, CONFIG.REFRESH_INTERVAL);

  input.addEventListener("input", updatePotentialPayout);
  input.addEventListener("change", updatePotentialPayout);

  input.addEventListener("keypress", (e) => {
    if (e.key === "Enter" && !isPlacing) {
      placePrediction();
    }
  });

  console.log("Prediction Market MiniApp initialized");
}

if (document.readyState === "loading") {
  document.addEventListener("DOMContentLoaded", init);
} else {
  init();
}
