const tokenValues = {
  "lightning.actionSprite": "/assets/icons/action-sprite/svg/symbols.svg",
  "lightning.actionSpriteRtl": "/assets/icons/action-sprite/svg/symbols.svg",
  "lightning.customSprite": "/assets/icons/custom-sprite/svg/symbols.svg",
  "lightning.customSpriteRtl": "/assets/icons/custom-sprite/svg/symbols.svg",
  "lightning.doctypeSprite": "/assets/icons/doctype-sprite/svg/symbols.svg",
  "lightning.doctypeSpriteRtl": "/assets/icons/doctype-sprite/svg/symbols.svg",
  "lightning.standardSprite": "/assets/icons/standard-sprite/svg/symbols.svg",
  "lightning.standardSpriteRtl": "/assets/icons/standard-sprite/svg/symbols.svg",
  "lightning.utilitySprite": "/assets/icons/utility-sprite/svg/symbols.svg",
  "lightning.utilitySpriteRtl": "/assets/icons/utility-sprite/svg/symbols.svg",
};

let providedConfig = null;

function configProviderService(serviceAPI = null) {
  providedConfig = serviceAPI || null;
  return { name: "lightning-config-provider" };
}

function callProvider(name, fallback, args = []) {
  const implementation = providedConfig?.[name];
  if (typeof implementation !== "function") {
    return fallback;
  }
  try {
    return implementation(...args);
  } catch (_err) {
    return fallback;
  }
}

export function getPathPrefix() {
  return callProvider("getPathPrefix", "", []);
}

export function getToken(name) {
  return callProvider("getToken", tokenValues[name] || "", [name]) || tokenValues[name] || "";
}

export function getIconSvgTemplates() {
  return providedConfig?.iconSvgTemplates || null;
}

export function getOneConfig() {
  return providedConfig?.getOneConfig?.() || { densitySetting: "" };
}

export function getLocalizationService() {
  return providedConfig?.getLocalizationService?.() || {
    formatDate(value) {
      return value ? new Date(value).toLocaleDateString() : "";
    },
    formatTime(value) {
      return value ? new Date(value).toLocaleTimeString() : "";
    },
    formatDateTimeUTC(value) {
      return value ? new Date(value).toISOString() : "";
    },
    parseDateTime(value) {
      return value ? new Date(value) : null;
    },
    parseDateTimeUTC(value) {
      return value ? new Date(value) : null;
    },
    parseDateTimeISO8601(value) {
      return value ? new Date(value) : null;
    },
    UTCToWallTime(value, _timezone, callback) {
      callback?.(value ? new Date(value) : value);
    },
    WallTimeToUTC(value, _timezone, callback) {
      callback?.(value ? new Date(value) : value);
    },
    translateToLocalizedDigits(value) {
      return value;
    },
    translateFromLocalizedDigits(value) {
      return value;
    },
    getNumberFormat(options = {}) {
      return new Intl.NumberFormat(undefined, options);
    },
  };
}

configProviderService.getPathPrefix = getPathPrefix;
configProviderService.getToken = getToken;
configProviderService.getIconSvgTemplates = getIconSvgTemplates;
configProviderService.getLocalizationService = getLocalizationService;
configProviderService.getOneConfig = getOneConfig;

export default configProviderService;
