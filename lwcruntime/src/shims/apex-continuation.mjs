export function createContinuation(method, params = {}) {
  return {
    method: String(method || ""),
    params: normalizeParams(params),
    status: "simulated",
    supportTier: "supported-local-simulated",
  };
}

export function invokeContinuation(method, params = {}) {
  return Promise.resolve(createContinuation(method, params));
}

export function resumeContinuation(continuation, callbackValue = {}) {
  const request = continuation && typeof continuation === "object"
    ? continuation
    : createContinuation(continuation);
  return Promise.resolve({
    ...request,
    callbackValue,
    status: "simulated",
    supportTier: "supported-local-simulated",
  });
}

export function simulatedContinuationError(method, message = "Apex continuation scheduling is simulated in local Glade.") {
  return {
    ok: false,
    status: 501,
    statusText: "NOT_IMPLEMENTED",
    body: {
      message,
      errorCode: "GLADELWC120",
      method: String(method || ""),
      supportTier: "supported-local-simulated",
    },
  };
}

function normalizeParams(params) {
  if (!params || typeof params !== "object" || Array.isArray(params)) {
    return {};
  }
  return { ...params };
}
