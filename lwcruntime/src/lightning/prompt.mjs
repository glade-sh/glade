import { createBaseComponent, renderDialogNotice } from "./base.mjs";
import { openFeedback } from "../shell/overlay.mjs";

const LightningPrompt = createBaseComponent("lightning-prompt", renderDialogNotice("prompt"));
LightningPrompt.open = function open(options = {}) { return openFeedback("prompt", options); };

export default LightningPrompt;
