export const CurrentPageReference = {};

function toURL(pageReference = {}) {
  const attrs = pageReference.attributes || {};
  switch (pageReference.type) {
    case "standard__recordPage":
      return `/lightning/r/${attrs.objectApiName || "Record"}/${attrs.recordId || ""}/${attrs.actionName || "view"}`;
    case "standard__objectPage":
      return `/lightning/o/${attrs.objectApiName || "Object"}/${attrs.actionName || "home"}`;
    case "standard__navItemPage":
      return `/lightning/n/${attrs.apiName || ""}`;
    case "standard__webPage":
      return attrs.url || "#";
    default:
      return "#";
  }
}

export const NavigationMixin = (Base) => class extends Base {
  Navigate(pageReference) {
    const url = toURL(pageReference);
    window.dispatchEvent(new CustomEvent("glade:navigate", { detail: { pageReference, url } }));
  }
  GenerateUrl(pageReference) {
    return Promise.resolve(toURL(pageReference));
  }
};

NavigationMixin.Navigate = "Navigate";
NavigationMixin.GenerateUrl = "GenerateUrl";

export function generateUrl(pageReference) {
  return Promise.resolve(toURL(pageReference));
}

export function navigate(pageReference) {
  const url = toURL(pageReference);
  window.dispatchEvent(new CustomEvent("glade:navigate", { detail: { pageReference, url } }));
}

export const CurrentPageReferenceAdapter = CurrentPageReference;
