package lwcbrowser

// Both browser entry points use the same mounting, focus and promise lifetime.
const modalModuleHelpersJS = `import { openModal, updateModalDisabled, closeModal } from "/lightning/runtime/shell/modal-overlay.js";`

const modalClassExtraJS = `  static open(options = {}) {
    return openModal(this, options);
  }
  get disableClose() {
    return this.__disableClose;
  }
  set disableClose(value) {
    this.__disableClose = value;
    updateModalDisabled(this, value);
  }
  close(result) {
    return closeModal(this, result);
  }
`
