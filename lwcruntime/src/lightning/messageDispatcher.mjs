let nextId = 1;
const handlers = new Map();
const domains = [];

export function clearDomains() {
  domains.splice(0, domains.length);
}
export function getDomains() {
  return domains.slice();
}
export function registerDomain(domain) {
  if (domain && !domains.includes(domain)) {
    domains.push(domain);
  }
}
export function unregisterDomain(domain) {
  const index = domains.indexOf(domain);
  if (index >= 0) {
    domains.splice(index, 1);
  }
}
export function setMessageEventHandled() {}
export function registerMessageHandler(handler) {
  const id = `glade-message-${nextId++}`;
  handlers.set(id, handler);
  return id;
}
export function unregisterMessageHandler(id) {
  handlers.delete(id);
}
export function dispatchEvent(event) {
  window.dispatchEvent(event);
}
export function createMessage(dispatcherId, event, params = {}) {
  return { dispatcherId, event, params };
}
export function postMessage(handler, message, domain, useObject) {
  void domain;
  void useObject;
  if (typeof handler === "function") {
    handler(message);
  }
}
