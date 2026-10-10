import { createBaseComponent } from "./base.mjs";
import Icon from "./icon.mjs";

function renderAvatar($api, $cmp) {
  const { h, t, c } = $api;
  const contents = $cmp.initials ? [h("abbr", {
    key: 1,
    className: "slds-avatar__initials",
    attrs: { title: $cmp.alternativeText },
  }, [t(String($cmp.initials))])] : [c("lightning-icon", Icon, {
    key: 2,
    className: "slds-icon_container",
    attrs: { title: $cmp.alternativeText },
    props: { iconName: $cmp.fallbackIconName, alternativeText: $cmp.alternativeText },
  }, [])];
  return [h("span", {
    key: 0,
    className: "slds-avatar slds-avatar_medium",
  }, contents)];
}

export default createBaseComponent("lightning-avatar", renderAvatar);
