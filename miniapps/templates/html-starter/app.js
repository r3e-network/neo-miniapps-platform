function getSDK() {
  const sdk = window.MiniAppSDK;
  const note = document.getElementById("sdk-note");
  if (!sdk) {
    note.style.display = "block";
    return null;
  }
  note.style.display = "none";
  return sdk;
}

function setOut(value) {
  const out = document.getElementById("out");
  out.textContent = typeof value === "string" ? value : JSON.stringify(value, null, 2);
}

async function run(label, fn) {
  try {
    setOut({ status: "running", action: label });
    const result = await fn();
    setOut({ status: "ok", action: label, result });
  } catch (err) {
    setOut({
      status: "error",
      action: label,
      error: String(err?.message ?? err),
    });
  }
}

document.getElementById("btn-check").addEventListener("click", () => {
  const sdk = getSDK();
  setOut({ installed: Boolean(sdk), sdk_keys: sdk ? Object.keys(sdk) : [] });
});

document.getElementById("btn-address").addEventListener("click", () => {
  run("wallet.getAddress", async () => {
    const sdk = getSDK();
    if (!sdk) throw new Error("MiniAppSDK not installed");
    return await sdk.wallet.getAddress();
  });
});

document.getElementById("btn-price").addEventListener("click", () => {
  run("datafeed.getPrice", async () => {
    const sdk = getSDK();
    if (!sdk) throw new Error("MiniAppSDK not installed");
    const symbol = String(document.getElementById("symbol").value || "").trim();
    if (!symbol) throw new Error("symbol required");
    return await sdk.datafeed.getPrice(symbol);
  });
});

document.getElementById("btn-rng").addEventListener("click", () => {
  run("rng.requestRandom", async () => {
    const sdk = getSDK();
    if (!sdk) throw new Error("MiniAppSDK not installed");
    const appId = String(document.getElementById("app-id").value || "").trim();
    if (!appId) throw new Error("app_id required");
    return await sdk.rng.requestRandom(appId);
  });
});

document.getElementById("btn-pay").addEventListener("click", () => {
  run("payments.payGAS + wallet.invokeIntent", async () => {
    const sdk = getSDK();
    if (!sdk) throw new Error("MiniAppSDK not installed");

    const appId = String(document.getElementById("app-id").value || "").trim();
    const amount = String(document.getElementById("pay-amount").value || "").trim();
    const memo = String(document.getElementById("pay-memo").value || "").trim();

    if (!appId) throw new Error("app_id required");
    if (!amount) throw new Error("amount required");

    const pay = await sdk.payments.payGAS(appId, amount, memo || undefined);

    if (!sdk.wallet.invokeIntent) {
      return {
        pay,
        note: "wallet.invokeIntent not available (host must submit pay.invocation)",
      };
    }

    const tx = await sdk.wallet.invokeIntent(pay.request_id);
    return { pay, tx };
  });
});

document.getElementById("btn-vote").addEventListener("click", () => {
  run("governance.vote + wallet.invokeIntent", async () => {
    const sdk = getSDK();
    if (!sdk) throw new Error("MiniAppSDK not installed");

    const appId = String(document.getElementById("app-id").value || "").trim();
    const proposalId = String(document.getElementById("proposal").value || "").trim();
    const amount = String(document.getElementById("vote-amount").value || "").trim();
    const support = String(document.getElementById("vote-support").value || "") === "true";

    if (!appId) throw new Error("app_id required");
    if (!proposalId) throw new Error("proposal_id required");
    if (!amount) throw new Error("neo_amount required");

    const vote = await sdk.governance.vote(appId, proposalId, amount, support);

    if (!sdk.wallet.invokeIntent) {
      return {
        vote,
        note: "wallet.invokeIntent not available (host must submit vote.invocation)",
      };
    }

    const tx = await sdk.wallet.invokeIntent(vote.request_id);
    return { vote, tx };
  });
});
