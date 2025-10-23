(params = {}, secrets = {}) => {
  if (!params.schedule) {
    throw new Error("schedule parameter (cron expression) is required");
  }

  const name = params.name || `Job-${Date.now()}`;
  const description = params.description || "Periodic automation job";
  const enabled = params.enabled !== false;

  const automationAction = Devpack.automation.schedule({
    name,
    schedule: params.schedule,
    description,
    enabled,
  });

  let triggerAction = null;
  if (params.registerTrigger === true) {
    triggerAction = Devpack.triggers.register({
      type: "cron",
      rule: params.schedule,
      config: { timezone: secrets.timezone || "UTC" },
    });
  }

  const response = {
    job: automationAction.asResult(),
    enabled,
  };
  if (triggerAction) {
    response.trigger = triggerAction.asResult();
  }

  return Devpack.respond.success(response);
}
