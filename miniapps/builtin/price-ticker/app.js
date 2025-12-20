/**
 * Price Ticker MiniApp
 * Real-time cryptocurrency price feeds for Neo N3
 * @version 1.0.0
 */

"use strict";

const CONFIG = {
  REFRESH_INTERVAL: 5000,
  DEFAULT_SYMBOL: "BTC-USD",
};

let sdk = null;
let autoTimer = null;
let currentSymbol = CONFIG.DEFAULT_SYMBOL;

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

/**
 * Set output display
 */
function setOutput(value) {
  const out = document.getElementById("out");
  out.textContent = typeof value === "string" ? value : JSON.stringify(value, null, 2);
}

/**
 * Show status message
 */
function showStatus(message, type) {
  const status = document.getElementById("status");
  status.textContent = sanitize(message);
  status.className = `status ${type}`;
}

/**
 * Clear status message
 */
function clearStatus() {
  const status = document.getElementById("status");
  status.textContent = "";
  status.className = "status";
}

/**
 * Set symbol and refresh
 */
function setSymbol(symbol) {
  currentSymbol = symbol;
  document.getElementById("symbol").value = symbol;

  document.querySelectorAll(".symbol-btn").forEach((btn) => {
    btn.classList.toggle("selected", btn.textContent === symbol);
  });

  refresh();
}

/**
 * Update price display
 */
function updatePriceDisplay(data) {
  document.getElementById("display-symbol").textContent = currentSymbol;

  if (data && data.price) {
    const price = parseFloat(data.price);
    document.getElementById("display-price").textContent = `$${formatNumber(price)}`;
    document.getElementById("display-meta").textContent = `Last updated: ${new Date().toLocaleTimeString()}`;
  } else {
    document.getElementById("display-price").textContent = "$0.00";
    document.getElementById("display-meta").textContent = "No data available";
  }
}

/**
 * Refresh price data
 */
async function refresh() {
  sdk = window.MiniAppSDK;

  const symbolInput = document.getElementById("symbol");
  const symbol = String(symbolInput.value || "").trim();

  if (!symbol) {
    showStatus("Symbol is required", "error");
    return;
  }

  currentSymbol = symbol;
  showStatus("Loading...", "loading");
  setOutput({ status: "loading", symbol });

  try {
    if (!sdk || !sdk.datafeed || typeof sdk.datafeed.getPrice !== "function") {
      showStatus("MiniAppSDK datafeed not available. Please open in the platform host.", "error");
      return;
    }
    const res = await sdk.datafeed.getPrice(symbol);
    setOutput(res);
    updatePriceDisplay(res);
    clearStatus();
  } catch (err) {
    const errorMsg = err.message || String(err);
    setOutput({ status: "error", error: sanitize(errorMsg) });
    showStatus(`Error: ${sanitize(errorMsg)}`, "error");
    console.error("Price fetch error:", err);
  }
}

/**
 * Toggle auto-refresh
 */
function toggleAuto() {
  const btn = document.getElementById("btn-auto");

  if (autoTimer) {
    clearInterval(autoTimer);
    autoTimer = null;
    btn.textContent = "Auto: Off";
    btn.classList.remove("active");
  } else {
    autoTimer = setInterval(refresh, CONFIG.REFRESH_INTERVAL);
    btn.textContent = "Auto: 5s";
    btn.classList.add("active");
    refresh();
  }
}

/**
 * Initialize the MiniApp
 */
async function init() {
  sdk = window.MiniAppSDK;

  document.getElementById("btn-refresh").addEventListener("click", refresh);
  document.getElementById("btn-auto").addEventListener("click", toggleAuto);

  document.getElementById("symbol").addEventListener("keydown", (e) => {
    if (e.key === "Enter") {
      e.preventDefault();
      refresh();
    }
  });

  // Initial refresh
  await refresh();

  console.log("Price Ticker MiniApp initialized");
}

if (document.readyState === "loading") {
  document.addEventListener("DOMContentLoaded", init);
} else {
  init();
}
