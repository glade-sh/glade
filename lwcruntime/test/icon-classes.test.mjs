import assert from "node:assert/strict";
import test from "node:test";
import { computeSvgClass } from "../src/lightning/iconUtils.mjs";

// lightning-icon size and variant map to SLDS svg classes; without a size
// class every icon falls back to the 2rem medium .slds-icon size.
test("lightning-icon svg classes follow size and variant", () => {
  assert.equal(computeSvgClass("utility:add", undefined, undefined), "slds-icon slds-icon-text-default");
  assert.equal(computeSvgClass("utility:add", "x-small"), "slds-icon slds-icon-text-default slds-icon_x-small");
  assert.equal(computeSvgClass("utility:add", "medium"), "slds-icon slds-icon-text-default");
  assert.equal(computeSvgClass("utility:warning", "small", "warning"), "slds-icon slds-icon-text-warning slds-icon_small");
  assert.equal(computeSvgClass("utility:close", "xx-small", "inverse"), "slds-icon slds-icon_xx-small");
  assert.equal(computeSvgClass("standard:account", "large"), "slds-icon slds-icon_large");
  assert.equal(computeSvgClass("standard:account", "bogus"), "slds-icon");
});
