(params = {}, secrets = {}) => {
  if (!params.endpoint) {
    throw new Error("endpoint parameter is required");
  }

  const secretToken = secrets.webhookToken || params.token || "";
  if (!secretToken) {
    throw new Error("webhook token must be supplied via params.token or secrets.webhookToken");
  }

  const trigger = Devpack.triggers.register({
    type: "webhook",
    rule: params.event || "function.execution",
    config: {
      url: params.endpoint,
      token: secretToken,
    },
    enabled: params.enabled !== false,
  });

  return Devpack.respond.success({
    trigger: trigger.asResult(),
    subscribedEvent: params.event || "function.execution",
  });
}
