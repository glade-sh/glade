// Component JavaScript and page-level tools have different views of base
// component internals. Keep the real host intact for styling and page access.
// Source and generated base components share this registry without requiring
// either implementation to import the compiled-component read helper.
const registryKey = Symbol.for("glade.component.privateShadowHosts");
const privateShadowHosts = globalThis[registryKey] ||= new WeakSet();
const privateShadowView = Object.freeze({ shadowRoot: null });

export function registerPrivateComponentShadow(host) {
  privateShadowHosts.add(host);
}

// Used only as the receiver of a compiled shadowRoot read. Other properties,
// DOM arguments and nullish optional-chain receivers keep their identity.
export function componentShadowRootReceiver(receiver) {
  return privateShadowHosts.has(receiver) ? privateShadowView : receiver;
}
