// Generate random bytes via Devpack runtime.
// Expects optional params.length (defaults to 32) and params.requestId.
export default function (params = {}) {
  const length = Number(params.length || 32);
  if (!Number.isFinite(length) || length <= 0 || length > 1024) {
    throw new Error("length must be between 1 and 1024");
  }

  const action = Devpack.random.generate({
    length,
    requestId: params.requestId || "",
  });

  return Devpack.respond.success({
    length,
    requestId: params.requestId || null,
    action: action.asResult({ label: "random_generate" }),
  });
}
