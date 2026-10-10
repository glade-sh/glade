import { registerDecorators } from "lwc";
import { createBaseComponent, renderModal } from "./base.mjs";
import { openModal, updateModalDisabled, closeModal } from "../shell/modal-overlay.mjs";

class LightningModal extends createBaseComponent("lightning-modal", renderModal) {
  static open(options = {}) { return openModal(this, options); }
  get disableClose() { return this.__disableClose; }
  set disableClose(value) {
    this.__disableClose = value;
    updateModalDisabled(this, value);
  }
  close(result) { return closeModal(this, result); }
}

registerDecorators(LightningModal, {
  publicProps: { disableClose: { config: 3 } },
  publicMethods: ["close"],
});

export default LightningModal;
