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
const sizes = new Set(["xx-small", "x-small", "small", "large"]);
const textVariants = new Set(["success", "warning", "error", "light"]);

// lightning-icon documents size (xx-small..large, default medium) and
// variant (inverse, success, warning, error, light) as the SLDS svg classes
// slds-icon_<size> and slds-icon-text-<variant>.
export function computeSvgClass(iconName, size, variant) {
  const classes = ["slds-icon"];
  const normalizedVariant = typeof variant === "string" ? variant.trim().toLowerCase() : "";
  if (iconName && getCategory(iconName) === "utility") {
    if (textVariants.has(normalizedVariant)) classes.push("slds-icon-text-" + normalizedVariant);
    else if (normalizedVariant !== "inverse") classes.push("slds-icon-text-default");
  }
  const normalizedSize = typeof size === "string" ? size.trim().toLowerCase() : "";
  if (sizes.has(normalizedSize)) classes.push("slds-icon_" + normalizedSize);
  return classes.join(" ");
}
export const getIconColor = () => null;
export const polyfill = () => {};
