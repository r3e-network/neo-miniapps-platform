(params = {}) => {
  if (!params.dataSourceId) {
    throw new Error("dataSourceId is required");
  }
  if (!params.schedule) {
    throw new Error("schedule is required");
  }

  const oracleAction = Devpack.oracle.createRequest({
    dataSourceId: params.dataSourceId,
    payload: {
      task: params.task || "price-check",
      context: params.context || {},
    },
  });

  const automationAction = Devpack.automation.schedule({
    name: params.jobName || `FollowUp-${Date.now()}`,
    schedule: params.schedule,
    description: "Follow up on oracle result",
    enabled: params.enabled !== false,
  });

  return Devpack.respond.success({
    scheduled: automationAction.asResult(),
    request: oracleAction.asResult(),
  });
}
