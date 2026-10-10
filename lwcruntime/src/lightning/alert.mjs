import { createBaseComponent, renderDialogNotice } from "./base.mjs";
import { openFeedback } from "../shell/overlay.mjs";

const LightningAlert = createBaseComponent("lightning-alert", renderDialogNotice("alert"));
LightningAlert.open = function open(options = {}) { return openFeedback("alert", options); };

export default LightningAlert;
