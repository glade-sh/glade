const channels = window.__gladeMessageChannels || new Map();
window.__gladeMessageChannels = channels;
const capturedMessages = window.__gladeMessages || [];
window.__gladeMessages = capturedMessages;

export const APPLICATION_SCOPE = Symbol("APPLICATION_SCOPE");

export class MessageContext {
  constructor(dataCallback) {
    this.dataCallback = dataCallback;
    this.context = createMessageContext();
  }

  connect() {
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
  return { id: `glade-message-context-${Math.random().toString(36).slice(2)}` };
}

export function releaseMessageContext(context) {
  for (const bucket of channels.values()) {
    for (const subscription of [...bucket]) {
      if (subscription.context === context) {
        bucket.delete(subscription);
      }
    }
  }
}

export function subscribe(context, channel, listener, options = {}) {
  const key = channelKey(channel);
  const bucket = channels.get(key) || new Set();
  const subscription = { key, context, listener, options };
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
  const key = channelKey(channel);
  capturedMessages.push({ key, message });
  const bucket = channels.get(key);
  if (!bucket) {
    return;
  }
  for (const subscription of [...bucket]) {
    if (typeof subscription.listener === "function") {
      subscription.listener(message);
    }
  }
  document.dispatchEvent(new CustomEvent("glade:message", { detail: { key, message, context } }));
}

export function getCapturedMessages() {
  return [...capturedMessages];
}

export function clearMessages() {
  capturedMessages.splice(0, capturedMessages.length);
  channels.clear();
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
