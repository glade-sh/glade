import { readCommunityContextQuiet } from "../shims/community.mjs";

export function applyCommunityHost(root = document.body, context = readCommunityContextQuiet()) {
  if (!root || !context?.site) {
    return context || {};
  }
  root.dataset.gladeCommunityShell = "true";
  root.dataset.gladeCommunitySite = context.site;
  root.dataset.gladeCommunityBasePath = context.basePath || "/s";
  root.dataset.gladeCommunityGuest = String(Boolean(context.guest));
  if (context.language) {
    root.dataset.gladeCommunityLanguage = context.language;
  }
  return context;
}
