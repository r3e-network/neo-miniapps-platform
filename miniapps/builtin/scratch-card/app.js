/**
 * Scratch Card MiniApp
 * Instant win scratch cards with on-chain randomness for Neo N3
 * @version 1.0.0
 */

"use strict";

const CONFIG = {
  APP_ID: "builtin-scratch-card",
  GAS_DECIMALS: 8,
  SYMBOLS: ["7", "*", "$", "+", "X", "O"],
  PRIZES: {
    7: 100,
    "*": 50,
    $: 10,
    "+": 5,
  },
};

let sdk = null;
let selectedPrice = 0.1;
let currentCard = null;
let revealedCells = [];
let hasCard = false;
let isBuying = false;
let stats = { cardsPlayed: 0, totalWon: 0, biggestWin: 0 };

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

async function requestRandomSymbols(count) {
  if (!sdk || !sdk.rng || typeof sdk.rng.requestRandom !== "function") {
    throw new Error("Randomness API not available");
  }
  const res = await sdk.rng.requestRandom(CONFIG.APP_ID);
  const bytes = hexToBytes(res?.randomness ?? "");
  return Array.from({ length: count }, (_, idx) =>
    CONFIG.SYMBOLS[randomIntFromBytes(bytes, idx * 2, CONFIG.SYMBOLS.length)],
  );
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
 * Select card type/price
 */
function selectCardType(price) {
  selectedPrice = price;
  document.querySelectorAll(".card-type-btn").forEach((btn) => {
    const isSelected = parseFloat(btn.dataset.price) === price;
    btn.classList.toggle("selected", isSelected);
    btn.setAttribute("aria-checked", isSelected ? "true" : "false");
  });
}

/**
 * Update stats display
 */
function updateStats() {
  document.getElementById("cards-played").textContent = stats.cardsPlayed;
  document.getElementById("total-won").textContent = formatNumber(stats.totalWon);
  document.getElementById("biggest-win").textContent = formatNumber(stats.biggestWin);
}

/**
 * Generate card symbols using randomness
 */
async function generateCard() {
  sdk = window.MiniAppSDK;

  if (!sdk || !sdk.rng) {
    throw new Error("MiniAppSDK RNG not available. Please open in the platform host.");
  }
  return await requestRandomSymbols(9);
}

/**
 * Check for winning combinations
 */
function checkWin(symbols) {
  const lines = [
    [0, 1, 2],
    [3, 4, 5],
    [6, 7, 8], // rows
    [0, 3, 6],
    [1, 4, 7],
    [2, 5, 8], // columns
    [0, 4, 8],
    [2, 4, 6], // diagonals
  ];

  let bestPrize = 0;
  let winningLine = null;
  let winningSymbol = null;

  for (const line of lines) {
    const [a, b, c] = line;
    if (symbols[a] === symbols[b] && symbols[b] === symbols[c]) {
      const symbol = symbols[a];
      const prize = CONFIG.PRIZES[symbol] || 0;
      if (prize > bestPrize) {
        bestPrize = prize;
        winningLine = line;
        winningSymbol = symbol;
      }
    }
  }

  return { prize: bestPrize, line: winningLine, symbol: winningSymbol };
}

/**
 * Buy a new scratch card
 */
async function buyCard() {
  if (isBuying) return;

  sdk = window.MiniAppSDK;
  if (!sdk || !sdk.payments) {
    showStatus("MiniAppSDK payments not available. Please open in the platform host.", "error");
    return;
  }

  try {
    isBuying = true;
    const btn = document.getElementById("btn-buy");
    btn.disabled = true;
    btn.textContent = "Buying...";
    showStatus("Purchasing card...", "loading");

    const amountText = formatAmountInput(selectedPrice);
    await submitPayment(amountText, "scratch-card");

    currentCard = await generateCard();
    revealedCells = [];
    hasCard = true;

    resetCardDisplay();
    clearStatus();

    btn.disabled = false;
    btn.textContent = "Buy Card";
  } catch (err) {
    const errorMsg = err.message || String(err);
    if (errorMsg.includes("insufficient")) {
      showStatus("Insufficient GAS balance", "error");
    } else if (errorMsg.includes("rejected") || errorMsg.includes("cancelled")) {
      showStatus("Transaction was cancelled", "error");
    } else {
      showStatus(`Error: ${sanitize(errorMsg)}`, "error");
    }
    console.error("Buy card error:", err);

    const btn = document.getElementById("btn-buy");
    btn.disabled = false;
    btn.textContent = "Buy Card";
  } finally {
    isBuying = false;
  }
}

/**
 * Reset card display
 */
function resetCardDisplay() {
  const cells = document.querySelectorAll(".scratch-cell");
  cells.forEach((cell, index) => {
    cell.textContent = "?";
    cell.className = "scratch-cell";
    cell.setAttribute("aria-label", `Cell ${index + 1}, not revealed`);
  });

  document.getElementById("result-box").className = "result-box";
  document.getElementById("btn-reveal").disabled = false;
}

/**
 * Reveal a single cell
 */
function revealCell(index) {
  if (!hasCard || !currentCard) {
    showStatus("Buy a card first!", "error");
    return;
  }

  if (revealedCells.includes(index)) return;

  const cell = document.querySelector(`.scratch-cell[data-index="${index}"]`);
  if (!cell) return;

  revealedCells.push(index);
  cell.textContent = currentCard[index];
  cell.classList.add("revealed");
  cell.setAttribute("aria-label", `Cell ${index + 1}, showing ${currentCard[index]}`);

  if (revealedCells.length === 9) {
    checkAndShowResult();
  }
}

/**
 * Reveal all cells at once
 */
function revealAll() {
  if (!hasCard || !currentCard) {
    showStatus("Buy a card first!", "error");
    return;
  }

  for (let i = 0; i < 9; i++) {
    if (!revealedCells.includes(i)) {
      revealCell(i);
    }
  }
}

/**
 * Check result and show prize
 */
function checkAndShowResult() {
  const result = checkWin(currentCard);
  const resultBox = document.getElementById("result-box");
  const resultText = document.getElementById("result-text");
  const resultAmount = document.getElementById("result-amount");

  stats.cardsPlayed++;

  if (result.prize > 0) {
    const winAmount = selectedPrice * result.prize;
    stats.totalWon += winAmount;
    if (winAmount > stats.biggestWin) stats.biggestWin = winAmount;

    resultBox.className = "result-box win";
    resultText.textContent = `3x ${result.symbol} - ${result.prize}x WIN!`;
    resultAmount.textContent = `+${formatNumber(winAmount)} GAS`;

    // Highlight winning cells
    if (result.line) {
      result.line.forEach((idx) => {
        const cell = document.querySelector(`.scratch-cell[data-index="${idx}"]`);
        if (cell) cell.classList.add("winner");
      });
    }
  } else {
    resultBox.className = "result-box lose";
    resultText.textContent = "No match - Try again!";
    resultAmount.textContent = `-${formatNumber(selectedPrice)} GAS`;
  }

  hasCard = false;
  document.getElementById("btn-reveal").disabled = true;

  updateStats();
  saveData();
}

/**
 * Save data to storage
 */
function saveData() {
  try {
    const data = JSON.stringify(stats);
    localStorage.setItem("scratch_stats", data);
  } catch (err) {
    console.warn("Could not save data:", err);
  }
}

/**
 * Load data from storage
 */
function loadData() {
  try {
    const data = localStorage.getItem("scratch_stats");

    if (data) {
      stats = JSON.parse(data);
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

  loadData();
  updateStats();
  resetCardDisplay();

  console.log("Scratch Card MiniApp initialized");
}

if (document.readyState === "loading") {
  document.addEventListener("DOMContentLoaded", init);
} else {
  init();
}
