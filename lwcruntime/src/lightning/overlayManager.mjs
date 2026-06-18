export const TYPE_TOAST_CONTAINER = "lightning-toast-container";
export const LWC_OVERLAY_ENGINE = "lwc";
export const LWC_OVERLAY_STARTING_ZINDEX = 9000;
export const LWC_TOAST_CONTAINER_STARTING_ZINDEX = 10000;
export const LWC_ZINDEX_INCREMENT = 2;
export const LWC_ZINDEX_OFFSET = 1;
export const LWC_OVERLAY_TYPES = Object.freeze({});
export const AURA_OVERLAY_ENGINE = "aura";
export const AURA_STARTING_ZINDEX = 9001;
export const AURA_ZINDEX_INCREMENT = 2;
export const AURA_OVERLAY_TYPES = {};

const overlays = [];

export function normalizeOverlayDetails(_engine, type, details = {}) {
  return { type, ...details };
}
export function addOverlayToSharedState(overlayObject) {
  overlays.push(overlayObject);
  return overlayObject;
}
export function removeOverlayFromSharedState(overlayObject) {
  const index = overlays.indexOf(overlayObject);
  if (index >= 0) {
    overlays.splice(index, 1);
  }
}
export function subscribeOverlay(_shouldCall, callback) {
  if (callback) {
    callback(overlays.slice());
  }
  return () => {};
}
export function getStatCount() {
  return overlays.length;
}
export function isLwcModalActive() {
  return overlays.length > 0;
}
