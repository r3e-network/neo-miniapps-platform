(params = {}) => {
  if (!params.dataSourceId) {
    throw new Error("dataSourceId parameter is required");
  }
  if (!params.symbol) {
    throw new Error("symbol parameter is required");
  }

  Devpack.oracle.createRequest({
    dataSourceId: params.dataSourceId,
    alternateSourceIds: params.alternateSourceIds,
    payload: {
      symbol: params.symbol,
      window: params.windowMinutes || 5,
    },
  });

  return Devpack.respond.success({
    symbol: params.symbol,
    requestedAt: new Date().toISOString(),
  });
}
