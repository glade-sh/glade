import { createBaseComponent } from "./base.mjs";

function isCoordinate(value, limit) {
  if (typeof value !== "number" && typeof value !== "string") return false;
  if (typeof value === "string" && value.trim() === "") return false;
  const number = Number(value);
  return Number.isFinite(number) && number >= -limit && number <= limit;
}

function renderFormattedLocation($api, $cmp) {
  const valid = isCoordinate($cmp.latitude, 90) && isCoordinate($cmp.longitude, 180);
  const text = valid ? `${$cmp.latitude}, ${$cmp.longitude}` : "";
  return [$api.h("bdi", { key: 0 }, [$api.t(text)])];
}

export default createBaseComponent("lightning-formatted-location", renderFormattedLocation);
