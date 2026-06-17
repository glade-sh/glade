import { createBaseComponent, renderModal } from "./base.mjs";

class LightningModal extends createBaseComponent("lightning-modal", renderModal) {
  static async open(options = {}) {
    const detail = { ...options };
    window.dispatchEvent(new CustomEvent("lightning__modalopen", { detail }));
    return options.result;
  }

  close(result) {
    this.dispatchEvent(new CustomEvent("close", { bubbles: true, composed: true, detail: { result } }));
  }
}

export default LightningModal;
