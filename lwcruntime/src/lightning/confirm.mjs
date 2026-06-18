export default class LightningConfirm {
  static open(options = {}) {
    window.dispatchEvent(new CustomEvent("gladeconfirm", { detail: options, bubbles: true, composed: true }));
    return Promise.resolve(true);
  }
}
