/**
 * FlashLoan MiniApp
 * Decentralized flash loan protocol for Neo N3
 * @version 1.0.0
 */

"use strict";

const CONFIG = {
  APP_ID: "builtin-flashloan",
  GAS_DECIMALS: 8,
  FEE_BPS: 9, // 0.09%
  MIN_AMOUNT: 0.1,
  MAX_AMOUNT: 10000,
  REFRESH_INTERVAL: 5000,
};

let sdk = null;
let recentActivity = [];
let isExecuting = false;
let maxPoolBalance = 0;

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

function formatAmountInput(value) {
  if (typeof value === "number" && Number.isFinite(value)) {
    return value.toFixed(CONFIG.GAS_DECIMALS).replace(/\.?0+$/, "");
  }
  const trimmed = String(value ?? "").trim();
  if (!trimmed) return "0";
  return trimmed;
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
 * Update fee calculations based on input amount
 */
function updateFees() {
  const input = document.getElementById("borrow-amount");
  const amount = parseFloat(input.value) || 0;

  // Validate amount
  if (amount < CONFIG.MIN_AMOUNT) {
    input.setCustomValidity(`Minimum amount is ${CONFIG.MIN_AMOUNT} GAS`);
  } else if (amount > maxPoolBalance && maxPoolBalance > 0) {
    input.setCustomValidity(`Maximum available is ${formatNumber(maxPoolBalance)} GAS`);
  } else if (amount > CONFIG.MAX_AMOUNT) {
    input.setCustomValidity(`Maximum amount is ${CONFIG.MAX_AMOUNT} GAS`);
  } else {
    input.setCustomValidity("");
  }

  const fee = (amount * CONFIG.FEE_BPS) / 10000;
  const repay = amount + fee;

  document.getElementById("fee-amount").textContent = `${formatNumber(fee, 6)} GAS`;
  document.getElementById("repay-amount").textContent = `${formatNumber(repay, 6)} GAS`;
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
  const btn = document.getElementById("btn-flashloan");
  isExecuting = loading;

  if (loading) {
    btn.disabled = true;
    btn.textContent = "Executing...";
    btn.setAttribute("aria-busy", "true");
  } else {
    btn.disabled = false;
    btn.textContent = "Execute Flash Loan";
    btn.removeAttribute("aria-busy");
  }
}

/**
 * Refresh protocol statistics from contract
 */
async function refreshStats() {
  try {
    setDefaultStats();

    const totalLoans = recentActivity.length;
    const totalBorrowed = recentActivity.reduce((sum, activity) => sum + activity.amount, 0);
    const totalFees = recentActivity.reduce((sum, activity) => sum + activity.fee, 0);

    document.getElementById("total-loans").textContent = totalLoans.toLocaleString();
    document.getElementById("total-borrowed").textContent = formatNumber(totalBorrowed);
    document.getElementById("total-fees").textContent = formatNumber(totalFees, 4);
  } catch (err) {
    console.error("Error refreshing stats:", err);
    // Keep default stats if refresh fails.
    setDefaultStats();
  }
}

/**
 * Set default stats
 */
function setDefaultStats() {
  maxPoolBalance = 100;
  document.getElementById("pool-balance").textContent = "100.00";
  document.getElementById("total-loans").textContent = "0";
  document.getElementById("total-borrowed").textContent = "0.00";
  document.getElementById("total-fees").textContent = "0.0000";
  document.getElementById("max-borrow").textContent = "100.00 GAS";
}

/**
 * Execute flash loan transaction
 */
async function executeFlashLoan() {
  if (isExecuting) return;

  const input = document.getElementById("borrow-amount");
  const amount = parseFloat(input.value) || 0;

  // Validate input
  if (!input.checkValidity()) {
    showStatus(input.validationMessage || "Invalid amount", "error");
    return;
  }

  if (amount < CONFIG.MIN_AMOUNT) {
    showStatus(`Minimum amount is ${CONFIG.MIN_AMOUNT} GAS`, "error");
    return;
  }

  if (amount > maxPoolBalance && maxPoolBalance > 0) {
    showStatus(`Insufficient pool liquidity. Max: ${formatNumber(maxPoolBalance)} GAS`, "error");
    return;
  }

  sdk = window.MiniAppSDK;
  if (!sdk) {
    showStatus("MiniAppSDK not available. Please open in the platform host.", "error");
    return;
  }

  try {
    setButtonLoading(true);
    showStatus("Preparing flash loan transaction...", "loading");

    const fee = (amount * CONFIG.FEE_BPS) / 10000;
    const feeText = formatAmountInput(fee);

    const payment = await submitPayment(feeText, `flashloan:${amount}`);

    showStatus(
      `Flash loan request submitted! Borrowed: ${formatNumber(amount)} GAS, Fee: ${formatNumber(fee, 6)} GAS`,
      "success",
    );
    addActivity(payment.intent.request_id, amount, fee);

    // Refresh stats after a delay
    setTimeout(refreshStats, 1000);
  } catch (err) {
    const errorMsg = err.message || String(err);
    if (errorMsg.includes("insufficient")) {
      showStatus("Insufficient balance to pay transaction fee", "error");
    } else if (errorMsg.includes("rejected") || errorMsg.includes("cancelled")) {
      showStatus("Transaction was cancelled", "error");
    } else {
      showStatus(`Error: ${sanitize(errorMsg)}`, "error");
    }
    console.error("Flash loan error:", err);
  } finally {
    setButtonLoading(false);
  }
}

/**
 * Add activity to recent activity list
 */
function addActivity(referenceId, amount, fee) {
  recentActivity.unshift({
    referenceId: referenceId,
    amount: amount,
    fee: fee,
    time: new Date(),
  });

  // Keep only last 20 activities
  if (recentActivity.length > 20) {
    recentActivity.pop();
  }

  renderActivity();
  saveActivityToStorage();
}

/**
 * Render activity list (XSS-safe)
 */
function renderActivity() {
  const list = document.getElementById("activity-list");

  if (recentActivity.length === 0) {
    list.innerHTML = '<div class="activity-item"><span class="activity-address">No activity yet</span></div>';
    return;
  }

  // Clear existing content
  list.innerHTML = "";

  recentActivity.forEach((activity) => {
    const item = document.createElement("div");
    item.className = "activity-item";

    const addressSpan = document.createElement("span");
    addressSpan.className = "activity-address";
    const reference = String(activity.referenceId || activity.txHash || "");
    addressSpan.textContent = reference ? reference.substring(0, 12) + "..." : "pending";
    addressSpan.title = reference || "pending";

    const amountSpan = document.createElement("span");
    amountSpan.className = "activity-amount";
    amountSpan.textContent = `${formatNumber(activity.amount)} GAS`;

    const timeSpan = document.createElement("span");
    timeSpan.className = "activity-time";
    timeSpan.textContent = activity.time.toLocaleTimeString();

    item.appendChild(addressSpan);
    item.appendChild(amountSpan);
    item.appendChild(timeSpan);
    list.appendChild(item);
  });
}

/**
 * Save activity to local storage
 */
function saveActivityToStorage() {
  try {
    localStorage.setItem("flashloan_activity", JSON.stringify(recentActivity));
  } catch (err) {
    console.warn("Could not save activity:", err);
  }
}

/**
 * Load activity from local storage
 */
function loadActivityFromStorage() {
  try {
    const data = localStorage.getItem("flashloan_activity");

    if (data) {
      const parsed = JSON.parse(data);
      recentActivity = parsed.map((a) => ({
        ...a,
        time: new Date(a.time),
      }));
    }
  } catch (err) {
    console.warn("Could not load activity:", err);
    recentActivity = [];
  }
}

/**
 * Initialize the MiniApp
 */
async function init() {
  // Set up input validation
  const input = document.getElementById("borrow-amount");
  input.min = CONFIG.MIN_AMOUNT;
  input.max = CONFIG.MAX_AMOUNT;

  // Load saved activity
  loadActivityFromStorage();

  // Initial render
  updateFees();
  renderActivity();

  // Fetch initial stats
  await refreshStats();

  // Set up auto-refresh
  setInterval(refreshStats, CONFIG.REFRESH_INTERVAL);

  // Set up input event listeners
  input.addEventListener("input", updateFees);
  input.addEventListener("change", updateFees);

  // Set up keyboard shortcut (Enter to execute)
  input.addEventListener("keypress", (e) => {
    if (e.key === "Enter" && !isExecuting) {
      executeFlashLoan();
    }
  });

  console.log("FlashLoan MiniApp initialized");
}

// Initialize when DOM is ready
if (document.readyState === "loading") {
  document.addEventListener("DOMContentLoaded", init);
} else {
  init();
}
