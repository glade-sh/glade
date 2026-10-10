import {
  CurrentPageReferenceAdapter,
  generatePageReferenceUrl,
  navigate,
} from "../shell/navigation-service.mjs";

export { CurrentPageReferenceAdapter, navigate };
export const CurrentPageReference = CurrentPageReferenceAdapter;
export const generateUrl = generatePageReferenceUrl;

export const NavigationMixin = (Base) => class extends Base {
  [NavigationMixin.Navigate](pageReference, replace) {
    navigate(pageReference, { replace }).catch(() => undefined);
  }

  [NavigationMixin.GenerateUrl](pageReference) {
    return generatePageReferenceUrl(pageReference);
  }
};

NavigationMixin.Navigate = Symbol("lightning/navigation.Navigate");
NavigationMixin.GenerateUrl = Symbol("lightning/navigation.GenerateUrl");
