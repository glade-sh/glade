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
export const getIconColor = () => null;
export const polyfill = () => {};
