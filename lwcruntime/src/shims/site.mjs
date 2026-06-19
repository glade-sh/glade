import { readCommunityContextQuiet, readCommunityValue } from "./community.mjs";

export function readSiteId() {
  return readCommunityValue("siteId", "");
}

export function readActiveLanguages() {
  const languages = readCommunityContextQuiet().activeLanguages;
  if (Array.isArray(languages) && languages.length > 0) {
    return languages.map((language) => ({
      code: language.code,
      label: language.label,
      active: language.active === undefined ? true : Boolean(language.active),
    }));
  }
  return [{ code: "en-US", label: "English", active: true }];
}
