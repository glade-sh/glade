import { readCommunityValue } from "./community.mjs";

export function readSiteId() {
  return readCommunityValue("siteId", "");
}
