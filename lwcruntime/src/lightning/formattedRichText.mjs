import { createBaseComponent, renderTextContainer } from "./base.mjs";

export default createBaseComponent("lightning-formatted-rich-text", renderTextContainer("span", "slds-rich-text-editor__output"));
