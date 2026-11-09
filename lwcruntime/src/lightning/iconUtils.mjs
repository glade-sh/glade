const spriteMap = {
  action: "/assets/icons/action-sprite/svg/symbols.svg",
  custom: "/assets/icons/custom-sprite/svg/symbols.svg",
  doctype: "/assets/icons/doctype-sprite/svg/symbols.svg",
  standard: "/assets/icons/standard-sprite/svg/symbols.svg",
  utility: "/assets/icons/utility-sprite/svg/symbols.svg",
};

export const isValidName = (iconName) => /^[A-Za-z]+:[A-Za-z]\w*$/.test(iconName || "");
export const getCategory = (iconName) => String(iconName || "").split(":")[0] || "";
export const getName = (iconName) => String(iconName || "").split(":")[1] || "";
export const getIconPath = (iconName) => {
  const category = getCategory(iconName);
  const name = getName(iconName);
  return `${spriteMap[category] || spriteMap.utility}#${name}`;
};
export const computeSldsClass = (iconName) => `slds-icon-${getCategory(iconName) || "utility"}-${(getName(iconName) || "placeholder").replace(/_/g, "-")}`;
// The size and variant modify the SVG rather than the component host.
export function computeSvgClass(iconName, size, variant) {
  const classes = ["slds-icon"];
  if (iconName && getCategory(iconName) === "utility") {
    const normalized = typeof variant === "string" ? variant.trim().toLowerCase() : "";
    if (!["inverse", "bare"].includes(normalized)) classes.push(`slds-icon-text-${["success", "warning", "error"].includes(normalized) ? normalized : "default"}`);
  }
  const normalizedSize = typeof size === "string" ? size.trim().toLowerCase() : "";
  if (["xx-small", "x-small", "small", "large"].includes(normalizedSize)) classes.push(`slds-icon_${normalizedSize}`);
  return classes.join(" ");
}
export const getIconColor = () => null;
export const polyfill = () => {};
