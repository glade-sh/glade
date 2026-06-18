export const urlTypes = { standard: "standard_webPage" };

export class LinkInfo {
  constructor(url, dispatcher = null) {
    this.url = url;
    this.dispatcher = dispatcher;
    Object.freeze(this);
  }
}

const providers = new WeakMap();

export function hasLinkProvider(element) {
  return providers.has(element);
}
export function isLinkProvider(element) {
  return providers.has(element);
}
export function registerLinkProvider(element, providerFn) {
  providers.set(element, providerFn);
}
export function unregisterLinkProvider(element) {
  providers.delete(element);
}
export function getLinkInfo(_element, stateRef = {}) {
  const url = stateRef?.url || stateRef?.href || "#";
  return Promise.resolve(new LinkInfo(url, null));
}
export function updateRawLinkInfo(element, info = {}) {
  if (element && info.url) {
    element.href = info.url;
  }
}
