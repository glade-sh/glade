const subscriptions = new Map();
const errorHandlers = new Set();
let debug = false;
let nextId = 1;

export async function subscribe(channel, replayId, callback) {
  const id = `emp-${nextId++}`;
  subscriptions.set(id, { id, channel, replayId, callback });
  return { id, channel, replayId };
}

export async function unsubscribe(subscription, callback) {
  subscriptions.delete(subscription?.id);
  const result = { successful: true, subscription };
  if (typeof callback === "function") {
    callback(result);
  }
  return result;
}

export function onError(callback) {
  if (typeof callback === "function") {
    errorHandlers.add(callback);
  }
}

export function setDebugFlag(flag) {
  debug = Boolean(flag);
}

export async function isEmpEnabled() {
  return true;
}

export function __gladePublish(channel, payload) {
  for (const sub of subscriptions.values()) {
    if (sub.channel === channel && typeof sub.callback === "function") {
      sub.callback(payload);
    }
  }
}

export function clearEmpSubscriptions() {
  subscriptions.clear();
  errorHandlers.clear();
  debug = false;
  nextId = 1;
}

export const __gladeEmpState = {
  subscriptions,
  errorHandlers,
  get debug() {
    return debug;
  },
};
