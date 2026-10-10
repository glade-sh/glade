const channels = window.__gladeMessageChannels || new Map();
window.__gladeMessageChannels = channels;
const capturedMessages = window.__gladeMessages || [];
window.__gladeMessages = capturedMessages;
const contexts = window.__gladeMessageContexts || new Set();
window.__gladeMessageContexts = contexts;
let nextContextId = window.__gladeMessageContextId || 1;

export const APPLICATION_SCOPE = Symbol("APPLICATION_SCOPE");

export class MessageContext {
  constructor(dataCallback) {
    this.dataCallback = dataCallback;
    this.context = createMessageContext();
  }

  connect() {
    if (!contexts.has(this.context)) {
      this.context = createMessageContext();
    }
    if (typeof this.dataCallback === "function") {
      this.dataCallback(this.context);
    }
  }

  update() {
    this.connect();
  }

  disconnect() {
    releaseMessageContext(this.context);
  }
}

export function createMessageContext() {
  const context = Symbol(`glade-message-context-${nextContextId++}`);
  contexts.add(context);
  window.__gladeMessageContextId = nextContextId;
  return context;
}

export function releaseMessageContext(context) {
  context = contextToken(context);
  contexts.delete(context);
  for (const bucket of channels.values()) {
    for (const subscription of [...bucket]) {
      if (subscription.context === context) {
        bucket.delete(subscription);
      }
    }
  }
}

export function subscribe(context, channel, listener, options = {}) {
  context = requireContext(context);
  if (typeof listener !== "function") {
    throw new Error("lightning/messageService: invalid listener function");
  }
  if (options !== null && typeof options !== "object") {
    throw new Error("lightning/messageService: invalid subscriberOptions. It must be an object.");
  }
  if (options?.scope !== undefined && options.scope !== APPLICATION_SCOPE) {
    throw new Error(`No scope definition found for provided scope ID: ${String(options.scope)}`);
  }
  const key = channelKey(channel);
  const bucket = channels.get(key) || new Set();
  const subscription = { key, context, listener, options: { ...options } };
  bucket.add(subscription);
  channels.set(key, bucket);
  return subscription;
}

export function unsubscribe(subscription) {
  if (!subscription) {
    return;
  }
  const bucket = channels.get(subscription.key);
  if (bucket) {
    bucket.delete(subscription);
  }
}

export function publish(context, channel, message) {
  context = requireContext(context);
  const key = channelKey(channel);
  capturedMessages.push({ key, message });
  const bucket = channels.get(key);
  if (!bucket) {
    return;
  }
  // Native in-page delivery uses JSON payload semantics, including omitted
  // undefined properties and null for non-finite numbers inside objects.
  const payload = message !== null && typeof message === "object"
    ? JSON.parse(JSON.stringify(message))
    : message;
  for (const subscription of [...bucket]) {
    subscription.listener(payload);
  }
  document.dispatchEvent(new CustomEvent("glade:message", { detail: { key, message: payload, context } }));
}

export function getCapturedMessages() {
  return [...capturedMessages];
}

export function clearMessages() {
  capturedMessages.splice(0, capturedMessages.length);
  channels.clear();
  nextContextId = 1;
  window.__gladeMessageContextId = nextContextId;
}

function channelKey(channel) {
  if (typeof channel === "string") {
    return channel;
  }
  if (channel && typeof channel === "object") {
    return String(channel.name || channel.messageChannelName || channel.channelName || channel.default?.name || "default");
  }
  return "default";
}

function contextToken(context) {
  return context instanceof MessageContext ? context.context : context;
}

function requireContext(context) {
  const token = contextToken(context);
  if (!contexts.has(token)) {
    throw new Error("lightning/messageService: invalid message context");
  }
  return token;
}
