import { setFeatureFlag } from "/lightning/vendor/engine-dom.js";

// Component insertion and removal are managed by the LWC renderer. Moving a
// rendered child through DOM APIs must not disconnect its wires.
setFeatureFlag("DISABLE_NATIVE_CUSTOM_ELEMENT_LIFECYCLE", true);

export * from "/lightning/vendor/engine-dom.js";
