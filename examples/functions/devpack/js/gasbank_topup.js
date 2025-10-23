(params = {}, secrets = {}) => {
  const wallet = params.wallet || secrets.defaultWallet || "";
  const amount = Number(params.amount || 0);

  if (!wallet) {
    throw new Error("wallet parameter (or defaultWallet secret) is required");
  }

  if (Number.isNaN(amount) || amount <= 0) {
    throw new Error("amount must be a positive number");
  }

  const ensureAction = Devpack.gasBank.ensureAccount({ wallet });
  const withdrawAction = Devpack.gasBank.withdraw({ amount, wallet, to: wallet });

  return Devpack.respond.success({
    wallet,
    amount,
    actions: [ensureAction.asResult(), withdrawAction.asResult()],
  });
}
