import { openFeedback } from "../shell/overlay.mjs";

export default class LightningConfirm {
  static open(options = {}) { return openFeedback("confirm", options); }
}
