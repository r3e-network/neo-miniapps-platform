(function (globalThis) {
  const VERSION = "0.6.0";
  let actionCounter = 0;
  const pendingActions = [];

  function clone(value) {
    if (Array.isArray(value)) {
      return value.map(clone);
    }
    if (value && typeof value === "object") {
      const copy = {};
      for (const key of Object.keys(value)) {
        copy[key] = clone(value[key]);
      }
      return copy;
    }
    return value;
  }

  function ensureObject(arg, defaultValue = {}) {
    if (!arg || typeof arg !== "object") {
      return defaultValue;
    }
    return clone(arg);
  }

  function recordAction(type, params) {
    const action = {
      id: `action_${++actionCounter}`,
      type,
      params: ensureObject(params),
    };
    pendingActions.push(action);
    return {
      id: action.id,
      type: action.type,
      params: clone(action.params),
      toJSON() {
        return { __devpack_ref__: true, id: action.id, type: action.type };
      },
      asResult(meta) {
        return {
          __devpack_ref__: true,
          id: action.id,
          type: action.type,
          meta: meta ? clone(meta) : undefined,
        };
      },
    };
  }

  const respond = {
    success(data, meta) {
      return {
        success: true,
        data: data === undefined ? null : clone(data),
        meta: meta === undefined ? null : clone(meta),
      };
    },
    failure(error, meta) {
      return {
        success: false,
        error: clone(error),
        meta: meta === undefined ? null : clone(meta),
      };
    },
  };

  const gasBank = {
    ensureAccount(options) {
      return recordAction("gasbank.ensureAccount", ensureObject(options));
    },
    withdraw(options) {
      return recordAction("gasbank.withdraw", ensureObject(options));
    },
    balance(options) {
      return recordAction("gasbank.balance", ensureObject(options));
    },
    listTransactions(options) {
      return recordAction("gasbank.listTransactions", ensureObject(options));
    },
  };

  const oracle = {
    createRequest(options) {
      return recordAction("oracle.createRequest", ensureObject(options));
    },
  };

  const random = {
    generate(options) {
      return recordAction("random.generate", ensureObject(options));
    },
  };

  const priceFeeds = {
    recordSnapshot(options) {
      return recordAction("pricefeed.recordSnapshot", ensureObject(options));
    },
  };

  const dataFeeds = {
    submitUpdate(options) {
      return recordAction("datafeeds.submitUpdate", ensureObject(options));
    },
  };

  const dataStreams = {
    publishFrame(options) {
      return recordAction("datastreams.publishFrame", ensureObject(options));
    },
  };

  const dataLink = {
    createDelivery(options) {
      return recordAction("datalink.createDelivery", ensureObject(options));
    },
  };

  const triggers = {
    register(options) {
      return recordAction("triggers.register", ensureObject(options));
    },
  };

  const automation = {
    schedule(options) {
      return recordAction("automation.schedule", ensureObject(options));
    },
  };

  const Devpack = {
    version: VERSION,
    respond,
    gasBank,
    oracle,
    random,
    priceFeeds,
    dataFeeds,
    dataStreams,
    dataLink,
    triggers,
    automation,
    context: {},
    setContext(ctx) {
      this.context = ensureObject(ctx);
    },
    __flushActions() {
      const copy = pendingActions.map(clone);
      pendingActions.length = 0;
      return copy;
    },
    __reset() {
      pendingActions.length = 0;
      actionCounter = 0;
    },
  };

  if (!globalThis.Devpack) {
    globalThis.Devpack = Devpack;
  } else {
    globalThis.Devpack.__reset();
    globalThis.Devpack.version = VERSION;
  }
})(typeof globalThis !== "undefined" ? globalThis : this);
