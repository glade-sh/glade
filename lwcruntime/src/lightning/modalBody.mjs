import { createBaseComponent, renderSlotContainer } from "./base.mjs";

export default createBaseComponent("lightning-modal-body", renderSlotContainer("div", "slds-modal__content"));
