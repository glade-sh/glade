package lwcbrowser

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/glade-sh/glade/internal/resource"
	"github.com/glade-sh/glade/internal/storage"
)

// SalesforceImportMap returns import-map entries for @salesforce/* and lightning/* modules.
// uiGraphqlApi preserves the additional spelling admitted by the native compiler.
func SalesforceImportMap() map[string]string {
	imports := map[string]string{
		"@glade/shell/app":                      "/lightning/runtime/shell/app.js",
		"@glade/shell/router":                   "/lightning/runtime/shell/router.js",
		"@glade/shell/contextPanel":             "/lightning/runtime/shell/context-panel.js",
		"@glade/shell/diagnostics":              "/lightning/runtime/shell/diagnostics.js",
		"@glade/slds":                           "/lightning/runtime/slds/slds-loader.js",
		"@salesforce/apex":                      "/lightning/shims/core/apex.js",
		"@salesforce/apex/":                     "/lightning/shims/apex/",
		"@salesforce/apexContinuation":          "/lightning/shims/apexContinuation.js",
		"@salesforce/apexContinuation/":         "/lightning/shims/apexContinuation/",
		"@salesforce/client/":                   "/lightning/shims/client/",
		"@salesforce/client/formFactor":         "/lightning/shims/client/formFactor.js",
		"@salesforce/community/":                "/lightning/shims/community/",
		"@salesforce/community/basePath":        "/lightning/shims/community/basePath.js",
		"@salesforce/community/Id":              "/lightning/shims/community/Id.js",
		"@salesforce/contentAssetUrl/":          "/lightning/shims/contentAssetUrl/",
		"@salesforce/customPermission/":         "/lightning/shims/customPermission/",
		"@salesforce/i18n/":                     "/lightning/shims/i18n/",
		"@salesforce/label/":                    "/lightning/shims/label/",
		"@salesforce/messageChannel/":           "/lightning/shims/messageChannel/",
		"@salesforce/resourceUrl/":              "/lightning/shims/resourceUrl/",
		"@salesforce/schema/":                   "/lightning/shims/schema/",
		"@salesforce/site/":                     "/lightning/shims/site/",
		"@salesforce/site/activeLanguages":      "/lightning/shims/site/activeLanguages.js",
		"@salesforce/site/Id":                   "/lightning/shims/site/Id.js",
		"@salesforce/user/":                     "/lightning/shims/user/",
		"@salesforce/userPermission/":           "/lightning/shims/userPermission/",
		"experience/blockBuilderApi":            "/lightning/runtime/experience/blockBuilderApi.js",
		"experience/cmsDeliveryApi":             "/lightning/runtime/experience/cmsDeliveryApi.js",
		"experience/cmsEditorApi":               "/lightning/runtime/experience/cmsEditorApi.js",
		"lightning/":                            "/lightning/shims/lightning/",
		"lightning/actions":                     "/lightning/shims/lightning/actions.js",
		"lightning/alert":                       "/lightning/shims/lightning/alert.js",
		"lightning/ariaObserver":                "/lightning/shims/lightning/ariaObserver.js",
		"lightning/confirm":                     "/lightning/shims/lightning/confirm.js",
		"lightning/configProvider":              "/lightning/shims/lightning/configProvider.js",
		"lightning/context":                     "/lightning/shims/lightning/context.js",
		"lightning/datatableKeyboardMixins":     "/lightning/shims/lightning/datatableKeyboardMixins.js",
		"lightning/empApi":                      "/lightning/shims/lightning/empApi.js",
		"lightning/f6Controller":                "/lightning/shims/lightning/f6Controller.js",
		"lightning/fileDownload":                "/lightning/shims/lightning/fileDownload.js",
		"lightning/flowSupport":                 "/lightning/shims/lightning/flowSupport.js",
		"lightning/i18nCldrOptions":             "/lightning/shims/lightning/i18nCldrOptions.js",
		"lightning/i18nService":                 "/lightning/shims/lightning/i18nService.js",
		"lightning/iconUtils":                   "/lightning/shims/lightning/iconUtils.js",
		"lightning/internalLocalizationService": "/lightning/shims/lightning/internalLocalizationService.js",
		"lightning/mediaUtils":                  "/lightning/shims/lightning/mediaUtils.js",
		"lightning/messageDispatcher":           "/lightning/shims/lightning/messageDispatcher.js",
		"lightning/messageService":              "/lightning/shims/lightning/messageService.js",
		"lightning/navigation":                  "/lightning/shims/lightning/navigation.js",
		"lightning/overlayManager":              "/lightning/shims/lightning/overlayManager.js",
		"lightning/pageReferenceUtils":          "/lightning/shims/lightning/pageReferenceUtils.js",
		"lightning/platformResourceLoader":      "/lightning/shims/lightning/platformResourceLoader.js",
		"lightning/platformShowToastEvent":      "/lightning/shims/lightning/platformShowToastEvent.js",
		"lightning/platformUtilityBarApi":       "/lightning/runtime/lightning/platformUtilityBarApi.js",
		"lightning/platformWorkspaceApi":        "/lightning/shims/lightning/platformWorkspaceApi.js",
		"lightning/prompt":                      "/lightning/shims/lightning/prompt.js",
		"lightning/purifyLib":                   "/lightning/shims/lightning/purifyLib.js",
		"lightning/refresh":                     "/lightning/shims/lightning/refresh.js",
		"lightning/routingService":              "/lightning/shims/lightning/routingService.js",
		"lightning/showToastEvent":              "/lightning/shims/lightning/showToastEvent.js",
		"lightning/toast":                       "/lightning/shims/lightning/toast.js",
		"lightning/graphql":                     "/lightning/runtime/lightning/graphql.js",
		"lightning/uiAppsApi":                   "/lightning/runtime/lightning/uiAppsApi.js",
		"lightning/uiGraphQLApi":                "/lightning/runtime/lightning/uiGraphQLApi.js",
		"lightning/uiGraphqlApi":                "/lightning/runtime/lightning/uiGraphQLApi.js",
		"lightning/uiLearningPlatformApi":       "/lightning/runtime/lightning/uiLearningPlatformApi.js",
		"lightning/uiLayoutApi":                 "/lightning/shims/lightning/uiLayoutApi.js",
		"lightning/uiListApi":                   "/lightning/shims/lightning/uiListApi.js",
		"lightning/uiListsApi":                  "/lightning/runtime/lightning/uiListsApi.js",
		"lightning/uiObjectInfoApi":             "/lightning/shims/lightning/uiObjectInfoApi.js",
		"lightning/uiRelatedListApi":            "/lightning/shims/lightning/uiRelatedListApi.js",
		"lightning/uiRecordApi":                 "/lightning/shims/lightning/uiRecordApi.js",
		"lightning/utils":                       "/lightning/shims/lightning/utils.js",
	}
	for key, value := range SupportedLightningBaseComponentSpecifiers() {
		imports[key] = value
	}
	return imports
}

func ClientModuleJS(property string) string {
	switch property {
	case "formFactor":
		return `function readFormFactor() {
  const node = document.getElementById("glade-lwc-context");
  if (!node) {
    return "Large";
  }
  try {
    const context = JSON.parse(node.textContent || "{}");
    return context.formFactor || "Large";
  } catch (_err) {
    return "Large";
  }
}
export default readFormFactor();
`
	default:
		return unsupportedModuleJS("Unsupported @salesforce/client property: " + property)
	}
}

func ConfigProviderModuleJS() string {
	return `const tokenValues = {
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
const defaultOneConfig = {
  densitySetting: "",
};
const monthNames = [
  "January",
  "February",
  "March",
  "April",
  "May",
  "June",
  "July",
  "August",
  "September",
  "October",
  "November",
  "December",
];
function configProviderService(serviceAPI = null) {
  providedConfig = serviceAPI || null;
  return { name: "lightning-config-provider" };
}
function callProvider(name, fallback, args = []) {
  const implementation = providedConfig && providedConfig[name];
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
  return providedConfig && providedConfig.iconSvgTemplates || null;
}
export function getOneConfig() {
  const configured = providedConfig && providedConfig.getOneConfig;
  if (typeof configured === "function") {
    return configured() || defaultOneConfig;
  }
  if (configured && typeof configured === "object") {
    return configured;
  }
  return defaultOneConfig;
}
function isDate(value) {
  return Object.prototype.toString.call(value) === "[object Date]" && !Number.isNaN(value.getTime());
}
function toDate(value) {
  if (!value) {
    return null;
  }
  if (isDate(value)) {
    return new Date(value.getTime());
  }
  if (typeof value === "number") {
    const parsedNumber = new Date(value);
    return isDate(parsedNumber) ? parsedNumber : null;
  }
  if (typeof value !== "string") {
    return null;
  }
  const trimmed = value.trim();
  if (!trimmed) {
    return null;
  }
  if (/^\d{2}:\d{2}(:\d{2})?(\.\d+)?(([+-]\d\d:\d\d)|Z)?$/i.test(trimmed)) {
    const time = trimmed.endsWith("Z") || /[+-]\d\d:\d\d$/i.test(trimmed) ? trimmed : trimmed + "Z";
    const parsedTime = new Date("1970-01-01T" + time);
    return isDate(parsedTime) ? parsedTime : null;
  }
  if (/^\d{4}-\d{2}-\d{2}$/.test(trimmed)) {
    const parsedDate = new Date(trimmed + "T00:00:00.000Z");
    return isDate(parsedDate) ? parsedDate : null;
  }
  const parsed = new Date(trimmed);
  return isDate(parsed) ? parsed : null;
}
function pad(value, width = 2) {
  return String(value).padStart(width, "0");
}
function dateParts(date, utc = false) {
  return {
    year: utc ? date.getUTCFullYear() : date.getFullYear(),
    month: (utc ? date.getUTCMonth() : date.getMonth()) + 1,
    day: utc ? date.getUTCDate() : date.getDate(),
    hours: utc ? date.getUTCHours() : date.getHours(),
    minutes: utc ? date.getUTCMinutes() : date.getMinutes(),
    seconds: utc ? date.getUTCSeconds() : date.getSeconds(),
    milliseconds: utc ? date.getUTCMilliseconds() : date.getMilliseconds(),
  };
}
function formatDateValue(value, format = "MMM d, yyyy", utc = false) {
  const date = toDate(value);
  if (!date) {
    return new Date("");
  }
  const parts = dateParts(date, utc);
  switch (format) {
    case "YYYY-MM-DD":
    case "yyyy-MM-dd":
      return parts.year + "-" + pad(parts.month) + "-" + pad(parts.day);
    case "M/d/yyyy":
      return parts.month + "/" + parts.day + "/" + parts.year;
    case "MMMM d, yyyy":
      return monthNames[parts.month - 1] + " " + parts.day + ", " + parts.year;
    case "MMM d, yyyy":
    default:
      return monthNames[parts.month - 1].slice(0, 3) + " " + parts.day + ", " + parts.year;
  }
}
function formatTimeValue(value, format = "h:mm:ss a", utc = false) {
  const date = toDate(value);
  if (!date) {
    return new Date("");
  }
  const parts = dateParts(date, utc);
  if (format === "HH:mm:ss.SSS") {
    return pad(parts.hours) + ":" + pad(parts.minutes) + ":" + pad(parts.seconds) + "." + pad(parts.milliseconds, 3);
  }
  const twelveHour = ((parts.hours + 11) % 12) + 1;
  const suffix = parts.hours >= 12 ? "PM" : "AM";
  if (format === "h:mm a") {
    return twelveHour + ":" + pad(parts.minutes) + " " + suffix;
  }
  return twelveHour + ":" + pad(parts.minutes) + ":" + pad(parts.seconds) + " " + suffix;
}
function parseFormattedDate(value, format) {
  if (typeof value !== "string") {
    return null;
  }
  const trimmed = value.trim();
  if (format === "YYYY-MM-DD" || format === "yyyy-MM-dd") {
    return toDate(trimmed);
  }
  const shortMatch = /^(\d{1,2})\/(\d{1,2})\/(\d{4})$/.exec(trimmed);
  if (shortMatch) {
    return toDate(shortMatch[3] + "-" + pad(shortMatch[1]) + "-" + pad(shortMatch[2]));
  }
  const textMatch = /^([A-Za-z]+)\s+(\d{1,2}),\s*(\d{4})$/.exec(trimmed);
  if (textMatch) {
    const month = monthNames.findIndex((name) => name.toLowerCase().startsWith(textMatch[1].toLowerCase()));
    if (month >= 0) {
      return toDate(textMatch[3] + "-" + pad(month + 1) + "-" + pad(textMatch[2]));
    }
  }
  return toDate(trimmed);
}
function parseFormattedTime(value) {
  if (typeof value !== "string") {
    return null;
  }
  const match = /^(\d{1,2}):(\d{2})(?::(\d{2})(?:\.(\d{1,3}))?)?\s*([AaPp][Mm])?$/.exec(value.trim());
  if (!match) {
    return toDate(value);
  }
  let hours = Number(match[1]);
  const minutes = Number(match[2] || 0);
  const seconds = Number(match[3] || 0);
  const milliseconds = Number((match[4] || "0").padEnd(3, "0"));
  const suffix = match[5] && match[5].toLowerCase();
  if (suffix === "pm" && hours < 12) {
    hours += 12;
  } else if (suffix === "am" && hours === 12) {
    hours = 0;
  }
  const date = new Date();
  date.setHours(hours, minutes, seconds, milliseconds);
  return isDate(date) ? date : null;
}
function startOf(date, unit) {
  const out = toDate(date);
  if (!out) {
    return null;
  }
  switch (unit) {
    case "day":
      out.setHours(0, 0, 0, 0);
      break;
    case "hour":
      out.setMinutes(0, 0, 0);
      break;
    case "minute":
      out.setSeconds(0, 0);
      break;
    case "second":
      out.setMilliseconds(0);
      break;
    default:
      break;
  }
  return out;
}
function unitToMilliseconds(unit) {
  switch (unit) {
    case "milliseconds":
    case "millisecond":
      return 1;
    case "seconds":
    case "second":
      return 1000;
    case "hours":
    case "hour":
      return 60 * 60 * 1000;
    case "days":
    case "day":
      return 24 * 60 * 60 * 1000;
    case "weeks":
    case "week":
      return 7 * 24 * 60 * 60 * 1000;
    case "months":
    case "month":
      return 30 * 24 * 60 * 60 * 1000;
    case "years":
    case "year":
      return 365 * 24 * 60 * 60 * 1000;
    case "minutes":
    case "minute":
    default:
      return 60 * 1000;
  }
}
function humanizeDuration(milliseconds, withSuffix = false) {
  const abs = Math.abs(milliseconds);
  const units = [
    ["year", 365 * 24 * 60 * 60 * 1000],
    ["month", 30 * 24 * 60 * 60 * 1000],
    ["day", 24 * 60 * 60 * 1000],
    ["hour", 60 * 60 * 1000],
    ["minute", 60 * 1000],
    ["second", 1000],
  ];
  let amount = 0;
  let unit = "second";
  for (const candidate of units) {
    if (abs >= candidate[1] || candidate[0] === "second") {
      unit = candidate[0];
      amount = Math.round(abs / candidate[1]);
      break;
    }
  }
  const text = amount + " " + unit + (amount === 1 ? "" : "s");
  if (!withSuffix) {
    return text;
  }
  return milliseconds > 0 ? "in " + text : text + " ago";
}
const localizationService = {
  isBefore(date1, date2, unit) {
    const left = startOf(date1, unit);
    const right = startOf(date2, unit);
    return Boolean(left && right && left.getTime() < right.getTime());
  },
  isAfter(date1, date2, unit) {
    const left = startOf(date1, unit);
    const right = startOf(date2, unit);
    return Boolean(left && right && left.getTime() > right.getTime());
  },
  formatDateTimeUTC(date) {
    const parsed = toDate(date);
    if (!parsed) {
      return new Date("");
    }
    return formatDateValue(parsed, "MMM d, yyyy", true) + ", " + formatTimeValue(parsed, "h:mm:ss a", true);
  },
  formatDate(dateString, format, locale) {
    void locale;
    return formatDateValue(dateString, format, false);
  },
  formatDateUTC(dateString, format, locale) {
    void locale;
    return formatDateValue(dateString, format, true);
  },
  formatTime(timeString, format) {
    return formatTimeValue(timeString, format, false);
  },
  parseDateTimeUTC(dateTimeString) {
    return toDate(dateTimeString && /Z|[+-]\d\d:\d\d$/i.test(dateTimeString) ? dateTimeString : String(dateTimeString || "") + "Z");
  },
  parseDateTimeISO8601(dateTimeString) {
    return toDate(dateTimeString);
  },
  parseDateTime(dateTimeString, format, strictMode) {
    void strictMode;
    if (format && /[hH]:?m/.test(format)) {
      return parseFormattedTime(dateTimeString);
    }
    return parseFormattedDate(dateTimeString, format);
  },
  UTCToWallTime(date, timezone, callback) {
    void timezone;
    if (typeof callback === "function") {
      callback(toDate(date) || date);
    }
  },
  WallTimeToUTC(date, timezone, callback) {
    void timezone;
    if (typeof callback === "function") {
      callback(toDate(date) || date);
    }
  },
  translateToOtherCalendar(date) {
    return date;
  },
  translateFromOtherCalendar(date) {
    return date;
  },
  translateToLocalizedDigits(input) {
    return input;
  },
  translateFromLocalizedDigits(input) {
    return input;
  },
  getNumberFormat(format) {
    try {
      return new Intl.NumberFormat(undefined, format && typeof format === "object" ? format : undefined);
    } catch (_err) {
      return { format: (value) => String(value) };
    }
  },
  duration(value, unit) {
    const milliseconds = Number(value || 0) * unitToMilliseconds(unit);
    return {
      milliseconds,
      asIn(targetUnit) {
        return milliseconds / unitToMilliseconds(targetUnit && targetUnit.name || targetUnit);
      },
      humanize(locale) {
        void locale;
        return humanizeDuration(milliseconds, true);
      },
    };
  },
  displayDuration(value, withSuffix) {
    if (value && typeof value.humanize === "function") {
      return value.humanize("en");
    }
    const milliseconds = value && typeof value.milliseconds === "number" ? value.milliseconds : Number(value || 0);
    return humanizeDuration(milliseconds, Boolean(withSuffix));
  },
};
export function getLocalizationService() {
  const configured = providedConfig && providedConfig.getLocalizationService;
  if (typeof configured === "function") {
    return configured() || localizationService;
  }
  return localizationService;
}
configProviderService.getPathPrefix = getPathPrefix;
configProviderService.getToken = getToken;
configProviderService.getIconSvgTemplates = getIconSvgTemplates;
configProviderService.getLocalizationService = getLocalizationService;
configProviderService.getOneConfig = getOneConfig;
export default configProviderService;
`
}

func ConfirmModuleJS() string {
	return `import { openFeedback } from "/lightning/runtime/shell/overlay.js";
export default class LightningConfirm {
  static open(options = {}) { return openFeedback("confirm", options); }
}
`
}

func AlertModuleJS() string {
	return LightningBaseComponentModuleJS("alert")
}

func PromptModuleJS() string {
	return LightningBaseComponentModuleJS("prompt")
}

func ToastModuleJS() string {
	return LightningBaseComponentModuleJS("toast")
}

func LightningUtilityModuleJS(name string) (string, bool) {
	switch normalizeLightningBaseComponentName(name) {
	case "ariaobserver":
		return `export default class AriaObserver {
  constructor(_target = null, _options = {}) {}
  connect() {}
  disconnect() {}
  observe() {}
  sync() {}
}
`, true
	case "context":
		return `export default class LightningContext {
  constructor() {
    this.value = null;
  }
  provide(value) {
    this.value = value;
  }
  consume() {
    return this.value;
  }
}
export function createContextProvider() {
  return () => {};
}
`, true
	case "datatablekeyboardmixins":
		return `export const baseNavigation = {};
`, true
	case "f6controller":
		return `export const DEFAULT_CONFIG = { navKey: "F6", f6RegionAttribute: "data-f6-region", f6RegionHighlightClass: "f6-highlight" };
export const getActiveElement = (element) => element && element.getRootNode && element.getRootNode().activeElement || document.activeElement;
export class F6Controller {
  constructor(config = DEFAULT_CONFIG) {
    this.config = config;
  }
  initialize() {}
  disable() {}
  enable() {}
  disconnect() {}
}
export const createF6Controller = () => new F6Controller();
export const getCurrentRegionAttributeName = () => DEFAULT_CONFIG.f6RegionAttribute;
export default F6Controller;
`, true
	case "filedownload":
		return `export function generateUrl(recordId) {
  return recordId ? "/lightning/r/ContentDocument/" + encodeURIComponent(recordId) + "/view" : undefined;
}
`, true
	case "i18ncldroptions":
		return `export default function intlDatetimeformatPattern(_pattern = "") {
  return {};
}
`, true
	case "i18nservice":
		return `function asDate(value) {
  const date = value instanceof Date ? value : new Date(value);
  return Number.isNaN(date.getTime()) ? new Date("") : date;
}
export function clearCache() {}
export function getDateTimeCLDRParser() {
  return { parse: (value) => asDate(value) };
}
export function getDateTimeFormat(options = {}) {
  return new Intl.DateTimeFormat(undefined, options);
}
export function getDateTimeISO8601Parser() {
  return { parse: (value) => asDate(value) };
}
export function getNumberFormat(options = {}) {
  return new Intl.NumberFormat(undefined, options);
}
export function getNumberParser() {
  return { parse: (value) => Number(String(value).replace(/,/g, "")) };
}
export function getRelativeTimeFormat(options = {}) {
  return new Intl.RelativeTimeFormat(undefined, options);
}
`, true
	case "iconsvgtemplates", "iconsvgtemplatesaction", "iconsvgtemplatesactionrtl", "iconsvgtemplatescustom", "iconsvgtemplatescustomrtl", "iconsvgtemplatesdoctype", "iconsvgtemplatesdoctypertl", "iconsvgtemplatesrtl", "iconsvgtemplatesstandard", "iconsvgtemplatesstandardrtl", "iconsvgtemplatesutility", "iconsvgtemplatesutilityrtl":
		return `export default {};
`, true
	case "iconutils":
		return `const spriteMap = {
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
  return (spriteMap[category] || spriteMap.utility) + "#" + name;
};
export const computeSldsClass = (iconName) => "slds-icon-" + (getCategory(iconName) || "utility") + "-" + (getName(iconName) || "placeholder").replace(/_/g, "-");
export const getIconColor = () => null;
export const polyfill = () => {};
`, true
	case "internallocalizationservice":
		return `export function formatDateTimeUTC(value) {
  return new Date(value).toISOString();
}
export function formatDateUTC(value) {
  return new Date(value).toISOString().slice(0, 10);
}
export function parseDateTimeUTC(value) {
  return new Date(value);
}
export function syncUTCToWallTime(value) {
  return new Date(value);
}
export function syncWallTimeToUTC(value) {
  return new Date(value);
}
export function addressFormat(parts = {}) {
  return [parts.street, parts.city, parts.province, parts.postalCode, parts.country].filter(Boolean).join(", ");
}
export function nameFormat(parts = {}) {
  return [parts.salutation, parts.firstName, parts.middleName, parts.lastName, parts.suffix, parts.informalName].filter(Boolean).join(" ");
}
`, true
	case "mediautils":
		return `export function processImage(input, _options = null) {
  if (!input) {
    return Promise.reject(new Error("Unable to read the input data."));
  }
  return Promise.resolve(input);
}
`, true
	case "messagedispatcher":
		return `let nextId = 1;
const handlers = new Map();
const domains = [];
export function clearDomains() { domains.splice(0, domains.length); }
export function getDomains() { return domains.slice(); }
export function registerDomain(domain) { if (domain && !domains.includes(domain)) domains.push(domain); }
export function unregisterDomain(domain) { const index = domains.indexOf(domain); if (index >= 0) domains.splice(index, 1); }
export function setMessageEventHandled() {}
export function registerMessageHandler(handler) {
  const id = "glade-message-" + nextId++;
  handlers.set(id, handler);
  return id;
}
export function unregisterMessageHandler(id) { handlers.delete(id); }
export function dispatchEvent(event) { window.dispatchEvent(event); }
export function createMessage(dispatcherId, event, params = {}) { return { dispatcherId, event, params }; }
export function postMessage(handler, message, domain, useObject) {
  void domain; void useObject;
  if (typeof handler === "function") handler(message);
}
`, true
	case "overlaymanager":
		return `export const TYPE_TOAST_CONTAINER = "lightning-toast-container";
export const LWC_OVERLAY_ENGINE = "lwc";
export const LWC_OVERLAY_STARTING_ZINDEX = 9000;
export const LWC_TOAST_CONTAINER_STARTING_ZINDEX = 10000;
export const LWC_ZINDEX_INCREMENT = 2;
export const LWC_ZINDEX_OFFSET = 1;
export const LWC_OVERLAY_TYPES = Object.freeze({});
export const AURA_OVERLAY_ENGINE = "aura";
export const AURA_STARTING_ZINDEX = 9001;
export const AURA_ZINDEX_INCREMENT = 2;
export const AURA_OVERLAY_TYPES = {};
const overlays = [];
export function normalizeOverlayDetails(_engine, type, details = {}) { return { type, ...details }; }
export function addOverlayToSharedState(overlayObject) { overlays.push(overlayObject); return overlayObject; }
export function removeOverlayFromSharedState(overlayObject) { const index = overlays.indexOf(overlayObject); if (index >= 0) overlays.splice(index, 1); }
export function subscribeOverlay(_shouldCall, callback) { if (callback) callback(overlays.slice()); return () => {}; }
export function getStatCount() { return overlays.length; }
export function isLwcModalActive() { return overlays.length > 0; }
`, true
	case "purifylib":
		return `export default function sanitizeHTML(dirty, _config = undefined) {
  const template = document.createElement("template");
  template.innerHTML = String(dirty || "");
  for (const node of template.content.querySelectorAll("script")) {
    node.remove();
  }
  return template.innerHTML;
}
`, true
	case "routingservice":
		return `export const urlTypes = { standard: "standard_webPage" };
export class LinkInfo {
  constructor(url, dispatcher = null) {
    this.url = url;
    this.dispatcher = dispatcher;
    Object.freeze(this);
  }
}
const providers = new WeakMap();
export function hasLinkProvider(element) { return providers.has(element); }
export function isLinkProvider(element) { return providers.has(element); }
export function registerLinkProvider(element, providerFn) { providers.set(element, providerFn); }
export function unregisterLinkProvider(element) { providers.delete(element); }
export function getLinkInfo(_element, stateRef = {}) {
  const url = stateRef && (stateRef.url || stateRef.href) || "#";
  return Promise.resolve(new LinkInfo(url, null));
}
export function updateRawLinkInfo(element, info = {}) {
  if (element && info.url) element.href = info.url;
}
`, true
	case "utils":
		return `export function classSet(initial = "") {
  const values = new Set(String(initial || "").split(/\s+/).filter(Boolean));
  return {
    add(value) {
      if (typeof value === "string") values.add(value);
      if (value && typeof value === "object") {
        for (const [key, enabled] of Object.entries(value)) {
          if (enabled) values.add(key);
        }
      }
      return this;
    },
    invert() { return this; },
    toString() { return Array.from(values).join(" "); },
  };
}
export function queryFocusable(root) {
  return Array.from(root && root.querySelectorAll ? root.querySelectorAll("a,button,input,select,textarea,[tabindex]") : []);
}
export function formatLabel(label, ...args) {
  return String(label || "").replace(/\{(\d+)\}/g, (_match, index) => String(args[Number(index)] ?? ""));
}
export function linkTextNodes(value) { return value; }
export function formatUrl(value) { return String(value || ""); }
`, true
	default:
		return "", false
	}
}

func PageReferenceUtilsModuleJS() string {
	return `export function encodeDefaultFieldValues(values = {}) {
  return Object.keys(values)
    .sort()
    .map((key) => encodeURIComponent(key) + "=" + encodeURIComponent(values[key] == null ? "" : String(values[key])))
    .join(",");
}
export function decodeDefaultFieldValues(value = "") {
  const out = {};
  for (const part of String(value || "").split(",")) {
    if (!part) {
      continue;
    }
    const index = part.indexOf("=");
    const key = index === -1 ? part : part.slice(0, index);
    const raw = index === -1 ? "" : part.slice(index + 1);
    out[decodeURIComponent(key)] = decodeURIComponent(raw);
  }
  return out;
}
`
}

func CustomPermissionModuleJS(name string) string {
	return fmt.Sprintf(`import { readCustomPermission } from "/lightning/runtime/shims/user-permission.js";
export const permissionName = %q;
export default readCustomPermission(permissionName);
`, strings.TrimSuffix(strings.TrimSpace(name), ".js"))
}

func UserPermissionModuleJS(name string) string {
	return fmt.Sprintf(`import { readUserPermission } from "/lightning/runtime/shims/user-permission.js";
export const permissionName = %q;
export default readUserPermission(permissionName);
`, strings.TrimSuffix(strings.TrimSpace(name), ".js"))
}

func ApexContinuationModuleJS() string {
	return `import {
  createContinuation,
  invokeContinuation as invokeLocalContinuation,
  resumeContinuation,
  simulatedContinuationError,
} from "/lightning/runtime/shims/apex-continuation.js";
export const supportTier = "supported-local-simulated";
export { createContinuation, resumeContinuation, simulatedContinuationError };
export function invokeContinuation(method, params = {}) {
  return Promise.resolve(invokeLocalContinuation(method, params));
}
export default invokeContinuation;
`
}

func ApexContinuationMethodModuleJS(name string) string {
	method := strings.TrimSuffix(strings.TrimSpace(name), ".js")
	return fmt.Sprintf(`import { invokeContinuation } from "/lightning/shims/apexContinuation.js";
export const methodName = %q;
export default function invoke(params = {}) {
  return invokeContinuation(methodName, params);
}
`, method)
}

var simulatedNativeAPIModules = map[string]struct{}{
	"analyticsWaveApi":             {},
	"cmsDeliveryApi":               {},
	"conversationToolkitApi":       {},
	"industriesEducationPublicApi": {},
	"mobileCapabilities":           {},
	"serviceCloudVoiceToolkitApi":  {},
	"serviceKnowledgeApi":          {},
}

func SimulatedNativeAPIModuleJS(name string) (string, bool) {
	token := strings.TrimSuffix(strings.TrimSpace(name), ".js")
	if _, ok := simulatedNativeAPIModules[token]; !ok {
		return "", false
	}
	return fmt.Sprintf(`export const moduleName = %q;
export const supportTier = "partial-local-simulated";
export function isAvailable() {
  return true;
}
export function invoke(methodName, params = {}) {
  return Promise.resolve({ moduleName, methodName, params, supportTier });
}
export default { moduleName, supportTier, isAvailable, invoke };
`, token), true
}

func ActionsModuleJS() string {
	return `export class CloseActionScreenEvent extends CustomEvent {
  constructor(options = {}) {
    super("lightning__actionsclosescreen", {
      bubbles: options.bubbles,
      composed: options.composed,
      cancelable: options.cancelable,
      detail: options.detail,
    });
  }
}
`
}

func FlowSupportModuleJS() string {
	return `export class FlowAttributeChangeEvent extends CustomEvent {
  constructor(attributeName, value) {
    super("lightning__flowattributechange", { bubbles: true, composed: true, detail: { property: attributeName, value } });
  }
}
function flowNavigationEvent(navigationTarget) {
  return class extends CustomEvent {
    constructor() {
      super("lightning__flownavigation", { bubbles: true, composed: true, detail: { navigationTarget } });
    }
  };
}
export const FlowNavigationNextEvent = flowNavigationEvent("NEXT");
export const FlowNavigationBackEvent = flowNavigationEvent("BACK");
export const FlowNavigationPauseEvent = flowNavigationEvent("PAUSE");
export const FlowNavigationFinishEvent = flowNavigationEvent("FINISH");
`
}

func RefreshModuleJS() string {
	return `const handlers = new Map();
const containers = new Map();
const registrations = new Map();
const documentRoots = new WeakMap();
const detachedRoot = { children: [] };
let nextRefreshHandle = 1;
// Captured named status bindings are undefined; refreshes resolve to 1 or 2.
export const RefreshComplete = undefined;
export const RefreshCompleteWithError = undefined;
export const RefreshError = undefined;
export class RefreshEvent extends CustomEvent {
  constructor() {
    super("lightning__refresh", { bubbles: true, composed: true, cancelable: true });
  }
}
function hostElement(context) {
  return context && (context.template?.host || context.hostElement || context);
}
function parentElement(element) {
  return element && (element.parentElement || element.parentNode || element.host);
}
function contains(ancestor, element) {
  for (let current = element; current; current = parentElement(current)) {
    if (current === ancestor) return true;
  }
  return false;
}
function refreshRoot(element) {
  const document = element.ownerDocument;
  if (!document) return detachedRoot;
  let root = documentRoots.get(document);
  if (!root) {
    root = { children: [] };
    documentRoots.set(document, root);
    // A hosted page remains a refresh view when it has no owned container.
    // Container listeners consume events before this default view receives them.
    document.addEventListener("lightning__refresh", (event) => {
      event.stopPropagation();
      refreshChildren(root.children.slice());
    });
  }
  return root;
}
function encloses(node, element, kind) {
  if (node.element === element) return node.kind === "container" && kind === "handler";
  return contains(node.element, element);
}
function register(context, callback, kind) {
  const element = hostElement(context);
  if (!element || typeof element.addEventListener !== "function" || typeof element.dispatchEvent !== "function") {
    throw new Error("Invalid contextElement. Must be an HTMLElement or LightningElement.");
  }
  if (typeof callback !== "function") {
    throw new Error("Invalid providerMethod. Must be of type Function.");
  }
  const registry = kind === "handler" ? handlers : containers;
  // Direct duplicate controls reject both the same and a fresh provider.
  if (registry.has(element)) {
    throw new Error("Invalid duplicate element registration. Element cannot be registered multiple times as a container or handler.");
  }
  let parent = refreshRoot(element);
  for (;;) {
    const ancestor = parent.children.find((node) => encloses(node, element, kind));
    if (!ancestor) break;
    parent = ancestor;
  }
  const node = { element, callback, kind, handle: nextRefreshHandle++, parent, children: [], active: true };
  // New registrations precede their peers. An ancestor registered after its
  // descendants adopts them in insertion order (the reverse-order controls).
  for (const child of parent.children.slice()) {
    if (encloses(node, child.element, child.kind)) {
      parent.children.splice(parent.children.indexOf(child), 1);
      node.children.unshift(child);
      child.parent = node;
    }
  }
  parent.children.unshift(node);
  registry.set(element, node);
  registrations.set(node.handle, node);
  if (kind === "container") {
    node.listener = (event) => {
      event.stopPropagation();
      refreshContainer(node);
    };
    element.addEventListener("lightning__refresh", node.listener);
  }
  return node.handle;
}
function unregister(value, kind) {
  const registry = kind === "handler" ? handlers : containers;
  const node = typeof value === "number" ? registrations.get(value)
    : registry.get(hostElement(value && value.element || value));
  if (!node) return;
  node.active = false;
  registrations.delete(node.handle);
  (node.kind === "handler" ? handlers : containers).delete(node.element);
  if (node.listener) node.element.removeEventListener("lightning__refresh", node.listener);
  const siblings = node.parent.children;
  siblings.splice(siblings.indexOf(node), 1);
  // Retain these children on the removed node for an already running refresh.
  // The live tree promotes them ahead of the remaining peers for later events.
  siblings.unshift(...node.children);
  for (const child of node.children) child.parent = node.parent;
}
export function registerRefreshHandler(element, handler) {
  return register(element, handler, "handler");
}
export function unregisterRefreshHandler(handle) {
  unregister(handle, "handler");
}
export function registerRefreshContainer(element, callback) {
  return register(element, callback, "container");
}
export function unregisterRefreshContainer(handle) {
  unregister(handle, "container");
}
function refreshContainer(node) {
  const children = node.children.slice();
  let resolveStatus;
  const status = new Promise((resolve) => { resolveStatus = resolve; });
  const callback = node.callback;
  try {
    callback(status);
  } catch {
    // A throwing container does not start its handlers (callback_throw).
    return Promise.resolve(2);
  }
  Promise.resolve().then(() => refreshChildren(children)).then(resolveStatus);
  return status;
}
function refreshChildren(nodes) {
  // Start sibling handlers together. Schedule ready branches in separate
  // microtask turns, without waiting for a sibling's pending descendants.
  let readyBranches = Promise.resolve();
  const visits = nodes.map((node) => {
    if (!node.active) return 1;
    if (node.kind === "container") return refreshContainer(node);
    const children = node.children.slice();
    const handler = node.callback;
    let result;
    try {
      result = handler();
      if (!result || typeof result.then !== "function") return 2;
    } catch {
      return 2;
    }
    return Promise.resolve(result).then((value) => {
      if (value === false) return 2;
      return new Promise((resolve) => {
        readyBranches = readyBranches.then(() => {
          refreshChildren(children).then(resolve);
        });
      });
    }, () => 2);
  });
  return Promise.all(visits).then((statuses) => statuses.includes(2) ? 2 : 1);
}
export function __gladeDispatchRefresh(context) {
  if (!context) return refreshChildren(detachedRoot.children.slice());
  const element = hostElement(context);
  const container = containers.get(element);
  if (container) return refreshChildren(container.children.slice());
  const handler = handlers.get(element);
  return refreshChildren(handler ? [handler] : []);
}
`
}

func EmpAPIModuleJS() string {
	return `export {
  __gladeEmpState,
  __gladePublish,
  clearEmpSubscriptions,
  isEmpEnabled,
  onError,
  setDebugFlag,
  subscribe,
  unsubscribe,
} from "/lightning/runtime/shell/emp-service.js";
`
}

func ApexWireModuleJS(className, methodName string) string {
	return fmt.Sprintf(
		`import { createApexWireAdapter } from "/lightning/shims/core/wire-adapter.js";`+
			`export default createApexWireAdapter(%q, %q);`,
		className, methodName,
	)
}

func LabelModuleJS(value string) string {
	return fmt.Sprintf("export default %q;\n", value)
}

func UserModuleJS(property, userID string) string {
	switch property {
	case "Id":
		if strings.TrimSpace(userID) == "" {
			userID = "005000000000001"
		}
		return defaultExportJS(salesforceUserID(userID))
	case "isGuest":
		return `import { readCommunityContext } from "/lightning/runtime/shims/community.js";
function readGuest() {
  return Boolean(readCommunityContext().guest);
}
export default readGuest();
`
	default:
		return unsupportedModuleJS("Unsupported @salesforce/user property: " + property)
	}
}

// The user virtual module exports the case-safe 18-character ID. Preserve
// existing 18-character IDs and unsupported inputs rather than coercing them.
func salesforceUserID(id string) string {
	if len(id) != 15 || storage.ValidateID(storage.ID(id)) != nil {
		return id
	}
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ012345"
	var suffix strings.Builder
	for chunk := 0; chunk < 3; chunk++ {
		mask := 0
		for bit := 0; bit < 5; bit++ {
			ch := id[chunk*5+bit]
			if ch >= 'A' && ch <= 'Z' {
				mask |= 1 << bit
			}
		}
		suffix.WriteByte(alphabet[mask])
	}
	return id + suffix.String()
}

// Community/site virtual modules are Experience Builder imports. User guest
// status and quiet context readers retain their standalone behavior.
const experienceContextGuardJS = `let experienceContext = {};
try {
  const node = typeof document === "undefined" ? null : document.getElementById("glade-lwc-context");
  experienceContext = node ? JSON.parse(node.textContent || "{}").community || {} : {};
} catch (_) {}
if (!experienceContext.site && !experienceContext.siteId && !experienceContext.networkId) {
  const error = new Error("EXPERIENCE_BUILDER_CONTEXT_REQUIRED");
  error.code = "EXPERIENCE_BUILDER_CONTEXT_REQUIRED";
  throw error;
}
`

// Owned black-box API 59/67 observations of common internationalization values.
// This is configuration data, not Salesforce implementation source.
//
//go:embed salesforce_i18n_data.json
var salesforceI18nData []byte

func CommunityModuleJS(property string) string {
	switch property {
	case "basePath":
		return experienceContextGuardJS + `import { readCommunityValue } from "/lightning/runtime/shims/community.js";
export default readCommunityValue("basePath", "/s");
`
	case "Id":
		return experienceContextGuardJS + `import { readCommunityValue } from "/lightning/runtime/shims/community.js";
export default readCommunityValue("networkId", "");
`
	case "Name":
		return `import { readCommunityValue } from "/lightning/runtime/shims/community.js";
export default readCommunityValue("name", "");
`
	case "Url":
		return `import { readCommunityValue } from "/lightning/runtime/shims/community.js";
export default readCommunityValue("url", "");
`
	default:
		return unsupportedModuleJS("Unsupported @salesforce/community property: " + property)
	}
}

func SiteModuleJS(property string) string {
	switch property {
	case "Id":
		return experienceContextGuardJS + `import { readSiteId } from "/lightning/runtime/shims/site.js";
export default readSiteId();
`
	case "activeLanguages":
		return experienceContextGuardJS + `import { readActiveLanguages } from "/lightning/runtime/shims/site.js";
export default readActiveLanguages();
`
	default:
		return unsupportedModuleJS("Unsupported @salesforce/site property: " + property)
	}
}

func I18nModuleJS(property string) string {
	if property == "timeZone" {
		return `function readTimeZone() {
  if (typeof document === "undefined") return "UTC";
  try {
    const node = document.getElementById("glade-lwc-context");
    const context = node ? JSON.parse(node.textContent || "{}") : {};
    return context.i18n?.timeZone || context.timeZone || "UTC";
  } catch (_) { return "UTC"; }
}
export default readTimeZone();
`
	}
	if property == "common.calendarData" || property == "common.digits" {
		var values map[string]json.RawMessage
		if err := json.Unmarshal(salesforceI18nData, &values); err == nil {
			return "export default " + string(values[property]) + ";\n"
		}
	}
	switch property {
	case "numberingSystem", "number.exponential", "number.superscriptingExponent", "number.timeSeparator":
		return "export default undefined;\n"
	}
	values := map[string]any{
		"currency":                  "USD",
		"dateTime.mediumDateFormat": "MMM d, yyyy",
		"dateTime.mediumTimeFormat": "h:mm:ss a",
		"dateTime.shortDateFormat":  "M/d/yyyy",
		"dateTime.longDateFormat":   "MMMM d, yyyy",
		"dateTime.shortTimeFormat":  "h:mm a",
		"dateTime.longTimeFormat":   "h:mm:ss a z",
		"defaultCalendar":           "gregorian",
		"defaultNumberingSystem":    "latn",
		"dir":                       "ltr",
		"firstDayOfWeek":            1,
		"isEasternNameStyle":        false,
		"lang":                      "en-US",
		"locale":                    "en-US",
		"number.currencyFormat":     "¤#,##0.00",
		"number.currencySymbol":     "$",
		"number.decimalSeparator":   ".",
		"number.groupingSeparator":  ",",
		"number.numberFormat":       "#,##0.###",
		"number.percentFormat":      "#,##0%",
		"number.plusSign":           "+",
		"number.minusSign":          "-",
		"number.perMilleSign":       "‰",
		"number.infinity":           "∞",
		"number.nan":                "NaN",
	}
	value, ok := values[property]
	if !ok {
		return unsupportedModuleJS("Unsupported @salesforce/i18n property: " + property)
	}
	return defaultExportJS(value)
}

func SchemaFieldModuleJS(objectName, fieldName string) string {
	return fmt.Sprintf(`const token = {
  fieldApiName: %q,
  objectApiName: %q,
};
Object.defineProperty(token, "toString", {value() { return %q; }});
export default token;
`, fieldName, objectName, objectName+"."+fieldName)
}

func SchemaObjectModuleJS(objectName string) string {
	return fmt.Sprintf(`const token = {
  objectApiName: %q,
  toString() { return %q; },
};
export default token;
`, objectName, objectName)
}

func ResourceURLModuleJS(url string) string {
	return fmt.Sprintf("export default %q;\n", url)
}

func ContentAssetURLModuleJS(url string) string {
	return fmt.Sprintf("export default %q;\n", url)
}

func MessageChannelModuleJS(name string) string {
	name = strings.TrimSuffix(strings.TrimSpace(name), ".js")
	return fmt.Sprintf(`const channel = {
  name: %q,
  messageChannelName: %q,
  toString() { return %q; },
};
export default channel;
`, name, name, name)
}

func NavigationModuleJS() string {
	return `import {
  CurrentPageReferenceAdapter,
  generatePageReferenceUrl as generateUrl,
  navigate,
} from "/lightning/runtime/shell/navigation-service.js";

export const supportedPageReferenceTypes = [
  "standard__recordPage",
  "standard__objectPage",
  "standard__recordRelationshipPage",
  "standard__navItemPage",
  "standard__app",
  "standard__namedPage",
  "standard__component",
  "standard__quickAction",
  "standard__webPage",
  "comm__namedPage",
  "comm__loginPage",
  "comm__managedContentPage",
  "comm__recordPage",
  "comm__recordRelationshipPage",
];
export const navigationDiagnosticCodes = ["GLADELWC040", "GLADELWC041", "GLADELWC042", "GLADELWC103"];
export const CurrentPageReference = CurrentPageReferenceAdapter;
export function NavigationMixin(Base) {
  return class extends Base {
    [NavigationMixin.Navigate](pageReference, replace) {
      navigate(pageReference, { replace }).catch(() => undefined);
    }
    [NavigationMixin.GenerateUrl](pageReference) {
      return generateUrl(pageReference);
    }
  };
}
NavigationMixin.Navigate = Symbol("lightning/navigation.Navigate");
NavigationMixin.GenerateUrl = Symbol("lightning/navigation.GenerateUrl");
export default NavigationMixin;
`
}

func PlatformWorkspaceAPIModuleJS() string {
	return `// GLADELWC072 glade-lwc-workbench activeRoute
import * as workspace from "/lightning/runtime/shell/workspace-service.js";
export {
  EnclosingTabId,
  IsConsoleNavigation,
  closeTab,
  configureWorkspace,
  disableTabClose,
  refreshTab,
  setTabHighlighted,
  setTabIcon,
  workspaceDiagnosticCodes,
} from "/lightning/runtime/shell/workspace-service.js";
function unsupported(method) {
  return new Error("Error: API " + "\x60" + method + "\x60" + " is not currently supported in this application.");
}
async function requireConsole(method) {
  if (!await workspace.isConsoleNavigation()) throw unsupported(method);
}
export async function getFocusedTabInfo() {
  await requireConsole("getFocusedTabInfo");
  return workspace.getFocusedTabInfo();
}
export async function getAllTabInfo() {
  await requireConsole("getAllTabInfo");
  return workspace.getAllTabInfo();
}
export async function getTabInfo(tabId) {
  if (tabId == null) throw new Error("error: unable to get info for a tab - missing tabId");
  await requireConsole("getTabInfo");
  return workspace.getTabInfo(tabId);
}
export async function focusTab(tabId) {
  if (tabId == null) throw new Error("error: unable to focus a tab - missing tabId");
  await requireConsole("focusTab");
  return workspace.focusTab(tabId);
}
export async function setTabLabel(tabId, label) {
  if (tabId == null || label == null) throw new Error("error: unable to set label to a tab - missing tabId or label");
  await requireConsole("setTabLabel");
  return workspace.setTabLabel(tabId, label);
}
export async function openTab(options = {}) {
  if (!await workspace.isConsoleNavigation()) {
    if (options?.pageReference?.type === "standard__navItemPage") return null;
    throw unsupported("openTab");
  }
  return workspace.openTab(options);
}
export async function openSubtab(parentTabId, options = {}) {
  if (parentTabId == null) throw new Error("error: unable to open a subtab - missing parent tab id");
  if (!await workspace.isConsoleNavigation()) {
    if (options?.pageReference?.type === "standard__navItemPage") return null;
    throw unsupported("openSubtab");
  }
  return workspace.openSubtab(parentTabId, options);
}
`
}

func UIRecordAPIModuleJS() string {
	return objectMetadataConfigJS() + `import { createFetchWireAdapter, createGetRecordWireAdapter } from "/lightning/shims/core/wire-adapter.js";
import {
  getRecordNotifyChange as notifyLegacyLDSCache,
  ldsCacheKey,
  writeLDSCache,
  notifyRecordUpdateAvailable as notifyLDSCache,
  refreshApex,
} from "/lightning/shims/core/lds-cache.mjs";
export { refreshApex };
export function notifyRecordUpdateAvailable(items) {
  const notifications = items.map((item) => item);
  invalidateNotifiedRecords(notifications);
  return notifyLDSCache(notifications);
}
export function getRecordNotifyChange(items = []) {
  invalidateNotifiedRecords(items);
  return notifyLegacyLDSCache(items);
}
function invalidateNotifiedRecords(items) {
  const ids = new Set();
  for (const item of Array.isArray(items) ? items : [items]) {
    const id = typeof item === "string" ? item : item?.recordId;
    if (typeof id === "string" && id.trim()) ids.add(id.trim());
  }
  // A refresh replaces only the active selection. Fields seeded by creation
  // but absent from that selection must remain stale until fetched again.
  for (const id of ids) invalidateRecordReads(id);
}
// Keep wire configuration suppression and delivery semantics, while the LDS
// surface supplies layout selection and its field-level record cache.
const records = new Map();
const recordCacheKeys = new Map();
function rememberRecord(data, mutation = false) {
  if (!data?.id || !data.fields) return;
  const key = data.id.slice(0, 15);
  const prior = records.get(key);
  const staleFields = new Set(prior?.staleFields);
  for (const name of Object.keys(data.fields)) staleFields.delete(name);
  records.set(key, {
    data: {...prior?.data, ...data, fields: {...prior?.data.fields, ...data.fields}},
    retainedFields: mutation ? Object.keys(data.fields) : prior?.retainedFields,
    staleFields,
  });
}
// Update responses may contain unwrapped fields and omit fields cleared by
// DML. Invalidate read values while keeping the retained selection used for
// validation, then let notifications/read requests provision fresh wrappers.
function invalidateRecordReads(recordId) {
  const id = String(recordId).slice(0, 15);
  const prior = records.get(id);
  if (prior) records.set(id, {...prior, staleFields: new Set(Object.keys(prior.data.fields))});
  for (const key of recordCacheKeys.get(id) || []) writeLDSCache(key, undefined);
  recordCacheKeys.delete(id);
}
function createLDSGetRecordAdapter() {
  const Parent = createGetRecordWireAdapter();
  function RecordAdapter(callback) {
    Parent.call(this, value => {
      if (value?.error?.status === 404 && this.body?.recordId) records.delete(String(this.body.recordId).slice(0, 15));
      rememberRecord(value?.data);
      callback(value);
    });
  }
  RecordAdapter.prototype = Object.create(Parent.prototype);
  RecordAdapter.prototype.constructor = RecordAdapter;
  RecordAdapter.prototype.refresh = function(options = {}) {
    if (this.body) {
      const body = {...this.body};
      if (this.config?.layoutTypes !== undefined) body.layoutTypes = this.config.layoutTypes;
      if (this.config?.modes !== undefined) body.modes = this.config.modes;
      const cached = records.get(String(body.recordId).slice(0, 15));
      // Qualification is validated by the server for a cache miss. A field
      // already held for this record is selected by name, as in the cold/primed
      // native controls; an uncached cross-object field remains an error.
      if (cached) body.fields = body.fields.map(ref => {
        const dot = ref.indexOf(".");
        const name = ref.slice(dot + 1);
        return dot >= 0 && Object.hasOwn(cached.data.fields, name) ? cached.data.apiName + "." + name : ref;
      });
      if (cached?.retainedFields) body.retainedFields = cached.retainedFields;
      this.body = body;
      const key = ldsCacheKey("/lightning/wire/getRecord", body);
      if (key !== this.cacheKey) this.pending += 1;
      this.cacheKey = key;
      const id = String(body.recordId).slice(0, 15);
      if (!recordCacheKeys.has(id)) recordCacheKeys.set(id, new Set());
      recordCacheKeys.get(id).add(key);
      // Native createRecord seeds fields before its promise settles. Publishing
      // a complete known selection through the shared cache makes a subsequent
      // configuration switch synchronous and retires any pending old request.
      // Forced notifications still reach the server and replace stale fields.
      if (cached && !options.force && body.fields.length && !body.layoutTypes?.length) {
        const refs = [...body.fields, ...(body.optionalFields || [])];
        const names = refs.map(ref => ref.slice(ref.indexOf(".") + 1));
        if (refs.every(ref => ref.startsWith(cached.data.apiName + ".")) && names.every(name => Object.hasOwn(cached.data.fields, name) && !cached.staleFields.has(name))) {
          const fields = Object.fromEntries(names.map(name => [name, cached.data.fields[name]]));
          writeLDSCache(key, {data: {...cached.data, fields}, error: undefined});
        }
      }
    }
    return Parent.prototype.refresh.call(this, options);
  };
  return RecordAdapter;
}
export const getRecord = createLDSGetRecordAdapter();
export const getRecordUi = createFetchWireAdapter("/lightning/wire/getRecordUi", (config) => {
  const recordIds = config && config.recordIds || [];
  if (!recordIds.length) {
    return null;
  }
  return compactBody({
    recordIds,
    fields: normalizeFields(config && config.fields),
    optionalFields: normalizeFields(config && config.optionalFields),
    layoutTypes: config && config.layoutTypes || [],
    modes: config && config.modes || [],
    recordTypeId: config && config.recordTypeId,
    formFactor: config && config.formFactor
  });
});
export const getRecords = createFetchWireAdapter("/lightning/wire/getRecords", (config) => {
  const records = config && config.records || [];
  if (!records.length || records.some((record) => !record?.recordIds?.length || record.recordIds.some((id) => !validRecordId(id)))) {
    return null;
  }
  return {
    records: records.map((record) => ({
    recordIds: record && record.recordIds || [],
    fields: normalizeFields(record && record.fields),
    optionalFields: normalizeFields(record && record.optionalFields)
  }))
  };
});
export const getObjectInfo = createFetchWireAdapter("/lightning/wire/getObjectInfo", (config) => {
  const apiName = metadataObjectApiName(config && config.objectApiName);
  return apiName ? { objectApiName: apiName } : null;
});
export const getObjectInfos = createFetchWireAdapter("/lightning/wire/getObjectInfos", (config) => {
  const objectApiNames = metadataObjectApiNames(config && config.objectApiNames);
  return objectApiNames ? { objectApiNames } : null;
});
export const getRecordCreateDefaults = createFetchWireAdapter("/lightning/wire/getRecordCreateDefaults", (config) => {
  const apiName = metadataObjectApiName(config && config.objectApiName);
  if (!apiName || (config.formFactor !== undefined && typeof config.formFactor !== "string")) {
    return null;
  }
  return compactBody({
    objectApiName: apiName,
    recordTypeId: config && config.recordTypeId,
    optionalFields: normalizeFields(config && config.optionalFields),
    formFactor: config && config.formFactor
  });
});
export const getPicklistValues = createFetchWireAdapter("/lightning/wire/getPicklistValues", (config) => {
  const apiName = metadataObjectApiName(config && config.objectApiName);
  const fieldName = fieldApiName(config && config.fieldApiName);
  if (typeof fieldName !== "string" || !fieldName || !metadataRecordTypeId(config && config.recordTypeId)) {
    return null;
  }
  return compactBody({
    objectApiName: apiName,
    fieldApiName: fieldName,
    recordTypeId: config && config.recordTypeId
  });
});
export const getPicklistValuesByRecordType = createFetchWireAdapter("/lightning/wire/getPicklistValuesByRecordType", (config) => {
  const apiName = metadataObjectApiName(config && config.objectApiName);
  if (!apiName || !metadataRecordTypeId(config && config.recordTypeId)) {
    return null;
  }
  return {
    objectApiName: apiName,
    recordTypeId: config && config.recordTypeId
  };
});
export const getRelatedListRecords = createFetchWireAdapter("/lightning/wire/getRelatedListRecords", (config) => {
  if (!(config && config.parentRecordId) || !config.relatedListId) {
    return null;
  }
  return {
    parentRecordId: config && config.parentRecordId,
    relatedListId: config && config.relatedListId,
    fields: normalizeFields(config && config.fields)
  };
});
export const getListUi = class GetListUiUnsupportedAdapter {
  constructor(dataCallback) {
    this.dataCallback = dataCallback;
  }
  connect() {}
  disconnect() {}
  update() {
    this.dataCallback({
      data: undefined,
      error: {
        code: "GLADELWC050",
        message: "GLADELWC050 getListUi unsupported locally; use getRelatedListRecords or local SOQL-backed Apex"
      }
    });
  }
};
function objectApiName(value) {
  if (value && typeof value === "object" && value.objectApiName) {
    return value.objectApiName;
  }
  return value;
}
function fieldApiName(value) {
  if (value && typeof value === "object") {
    if (value.fieldApiName && value.objectApiName) {
      return value.objectApiName + "." + value.fieldApiName;
    }
    return value.fieldApiName || "";
  }
  return value || "";
}
function normalizeFields(fields) {
  return (fields || []).map((field) => {
    if (field && typeof field === "object") {
      return field.objectApiName && field.fieldApiName ? field.objectApiName + "." + field.fieldApiName : field.fieldApiName;
    }
    return String(field);
  });
}
function compactBody(body) {
  const out = {};
  for (const [key, value] of Object.entries(body)) {
    if (value !== undefined) {
      out[key] = value;
    }
  }
  return out;
}
function post(endpoint, body) {
  return fetch(endpoint, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body || {})
  }).then((response) => response.json()).then((result) => {
    if (result && result.error) {
      if (result.error.errorType === "fetchResponse") {
        throw result.error;
      }
      const err = new Error(result.error.message || "Lightning Data Service request failed");
      err.body = result.error;
      throw err;
    }
    return result && result.data;
  });
}
export function createRecord(recordInput) {
  return post("/lightning/wire/createRecord", {
    apiName: recordInput && (recordInput.apiName || recordInput.objectApiName),
    fields: recordInput && recordInput.fields || {}
  }).then((data) => { rememberRecord(data, true); return notifyLDSCache(notificationItems(data)).then(() => data); });
}
export function updateRecord(recordInput, clientOptions) {
  const recordId = recordInput && recordInput.fields && recordInput.fields.Id;
  if (!validRecordId(recordId)) {
    return Promise.reject(new Error("Invalid recordInput"));
  }
  return post("/lightning/wire/updateRecord", {
    fields: recordInput.fields,
    ifUnmodifiedSince: clientOptions && clientOptions.ifUnmodifiedSince
  }).then((data) => { invalidateRecordReads(recordId); return notifyRecordUpdateAvailable(notificationItems(data, recordId)).then(() => data); });
}
export function deleteRecord(recordId) {
  if (!validRecordId(recordId)) {
    return Promise.reject(new Error('Invalid config for "deleteRecord"'));
  }
  return post("/lightning/wire/deleteRecord", { recordId })
    .then((data) => { invalidateRecordReads(recordId); records.delete(recordId.slice(0, 15)); return notifyRecordUpdateAvailable(notificationItems(data, recordId)).then(() => undefined); });
}
function validRecordId(value) {
  return typeof value === "string" && /^(?:[a-zA-Z0-9]{15}|[a-zA-Z0-9]{18})$/.test(value);
}
export async function __gladeRecordPickerSearch(config = {}) {
  return post("/lightning/wire/recordPickerSearch", config);
}
export function generateRecordInputForCreate(record, objectInfo) {
  const fields = recordFields(record, objectInfo, "createable");
  delete fields.Id;
  return {
    apiName: record.apiName,
    fields
  };
}
export function generateRecordInputForUpdate(record, objectInfo) {
  const fields = recordFields(record, objectInfo, "updateable");
  fields.Id = record.id;
  return { apiName: undefined, fields };
}
export function createRecordInputFilteredByEditedFields(recordInput, originalRecord) {
  const sourceFields = recordInput.fields;
  const out = { Id: originalRecord.id };
  for (const [name, value] of Object.entries(sourceFields)) {
    if (name === "Id") {
      continue;
    }
    const original = originalRecord && originalRecord.fields && originalRecord.fields[name];
    if (!sameValue(value, fieldValue(original))) {
      out[name] = value;
    }
  }
  return { apiName: recordInput.apiName, fields: out };
}
export function getFieldValue(record, field) {
  return recordFieldValue(record, field, "value");
}
export function getFieldDisplayValue(record, field) {
  return recordFieldValue(record, field, "displayValue");
}
function recordFieldValue(record, field, property) {
  let names;
  if (typeof field === "string") {
    const dot = field.indexOf(".");
    if (dot < 0) {
      throw new TypeError("Value does not include an object API name.");
    }
    names = field.slice(dot + 1).split(".");
  } else {
    names = field && field.fieldApiName ? field.fieldApiName.split(".") : [undefined];
  }
  let value = record;
  for (let index = 0; index < names.length; index++) {
    if (value && value.fields) {
      value = value.fields[names[index]];
      if (value && Object.prototype.hasOwnProperty.call(value, "value")) {
        value = value[index === names.length - 1 ? property : "value"];
      }
    }
  }
  return value;
}
function recordFields(record, objectInfo, accessProperty) {
  const fields = {};
  const source = record.fields;
  for (const [name, wrapped] of Object.entries(source)) {
    if (name === "Id" && accessProperty === "createable") {
      continue;
    }
    if (!fieldAllows(objectInfo, name, accessProperty)) {
      continue;
    }
    const value = wrapped.value;
    if (value === undefined || !recordInputValueSupported(value)) {
      continue;
    }
    fields[name] = value;
  }
  return fields;
}
function fieldAllows(objectInfo, name, accessProperty) {
  if (!objectInfo) {
    return true;
  }
  const field = (objectInfo.fields || {})[name];
  return !!field && field[accessProperty] === true;
}
function fieldValue(value) {
  if (value && typeof value === "object" && Object.prototype.hasOwnProperty.call(value, "value")) {
    return value.value;
  }
  return value;
}
function sameValue(left, right) {
  return JSON.stringify(left) === JSON.stringify(right);
}
function recordInputValueSupported(value) {
  return value === null || value === undefined || typeof value !== "object" || Array.isArray(value);
}
function notificationItems(record, fallbackId) {
  const ids = new Set();
  collectId(ids, fallbackId);
  collectId(ids, record && record.id);
  for (const field of Object.values(record && record.fields || {})) {
    collectId(ids, field && field.value);
  }
  return Array.from(ids).map((recordId) => ({ recordId }));
}
function collectId(ids, value) {
  if (typeof value === "string" && /^[a-zA-Z0-9]{15,18}$/.test(value)) {
    ids.add(value);
  }
}
	`
}

func UIListAPIModuleJS() string {
	return `import { createFetchWireAdapter } from "/lightning/shims/core/wire-adapter.js";
export const getListUi = createFetchWireAdapter("/lightning/wire/getListUi", (config = {}) => {
  if (typeof config.objectApiName !== "string" ||
      (config.listViewApiName != null && typeof config.listViewApiName !== "string")) return null;
  return Object.fromEntries(Object.entries(config).filter(([, value]) => value !== undefined));
});
`
}

func UILayoutAPIModuleJS() string {
	return objectMetadataConfigJS() + `import { createFetchWireAdapter } from "/lightning/shims/core/wire-adapter.js";
export const getLayout = createFetchWireAdapter("/lightning/wire/getLayout", (config) => {
  const apiName = metadataObjectApiName(config && config.objectApiName);
  if (!apiName || !["Full", "Compact"].includes(config.layoutType) || !["Create", "Edit", "View"].includes(config.mode)) {
    return null;
  }
  if (config.recordTypeId != null && !metadataRecordTypeId(config.recordTypeId)) {
    return null;
  }
  return compactBody({
    objectApiName: apiName,
    recordTypeId: config && config.recordTypeId,
    layoutType: config && config.layoutType,
    mode: config && config.mode,
    formFactor: config && config.formFactor
  });
});
function objectApiName(value) {
  if (value && typeof value === "object" && value.objectApiName) {
    return value.objectApiName;
  }
  return value;
}
function compactBody(body) {
  const out = {};
  for (const [key, value] of Object.entries(body)) {
    if (value !== undefined) {
      out[key] = value;
    }
  }
  return out;
}
	`
}

func UIObjectInfoAPIModuleJS() string {
	return objectMetadataConfigJS() + `import { createFetchWireAdapter } from "/lightning/shims/core/wire-adapter.js";
export const getObjectInfo = createFetchWireAdapter("/lightning/wire/getObjectInfo", (config) => {
  const apiName = metadataObjectApiName(config && config.objectApiName);
  return apiName ? { objectApiName: apiName } : null;
});
export const getObjectInfos = createFetchWireAdapter("/lightning/wire/getObjectInfos", (config) => {
  const objectApiNames = metadataObjectApiNames(config && config.objectApiNames);
  return objectApiNames ? { objectApiNames } : null;
});
export const getPicklistValues = createFetchWireAdapter("/lightning/wire/getPicklistValues", (config) => {
  const apiName = metadataObjectApiName(config && config.objectApiName);
  const fieldName = fieldApiName(config && config.fieldApiName);
  if (typeof fieldName !== "string" || !fieldName || !metadataRecordTypeId(config && config.recordTypeId)) {
    return null;
  }
  return compactBody({
    objectApiName: apiName,
    fieldApiName: fieldName,
    recordTypeId: config && config.recordTypeId
  });
});
export const getPicklistValuesByRecordType = createFetchWireAdapter("/lightning/wire/getPicklistValuesByRecordType", (config) => {
  const apiName = metadataObjectApiName(config && config.objectApiName);
  if (!apiName || !metadataRecordTypeId(config && config.recordTypeId)) {
    return null;
  }
  return {
    objectApiName: apiName,
    recordTypeId: config && config.recordTypeId
  };
});
function objectApiName(value) {
  if (value && typeof value === "object" && value.objectApiName) {
    return value.objectApiName;
  }
  return value;
}
function fieldApiName(value) {
  if (value && typeof value === "object") {
    if (value.fieldApiName && value.objectApiName) {
      return value.objectApiName + "." + value.fieldApiName;
    }
    return value.fieldApiName || "";
  }
  return value || "";
}
function compactBody(body) {
  const out = {};
  for (const [key, value] of Object.entries(body)) {
    if (value !== undefined) {
      out[key] = value;
    }
  }
  return out;
}
	`
}

// These validators belong to the UI metadata adapters. Record and relationship
// adapters retain their own configuration rules.
func objectMetadataConfigJS() string {
	return `function metadataObjectApiName(value) {
  const name = objectApiName(value);
  return typeof name === "string" && name.length ? name : undefined;
}
function metadataObjectApiNames(value) {
  const values = Array.isArray(value) ? value : typeof value === "string" ? [value] : [];
  const names = values.map(metadataObjectApiName);
  if (!names.length || names.some((name) => name === undefined)) {
    return null;
  }
  return [...new Set(names)];
}
function metadataRecordTypeId(value) {
  return typeof value === "string" && /^012[A-Za-z0-9]{12}(?:[A-Za-z0-9]{3})?$/.test(value);
}
`
}

func UIRelatedListAPIModuleJS() string {
	return `import { createFetchWireAdapter } from "/lightning/shims/core/wire-adapter.js";
export const getRelatedListRecords = createFetchWireAdapter("/lightning/wire/getRelatedListRecords", (config) => {
  if (!recordConfig(config) || !config.relatedListId) {
    return null;
  }
  return compactBody({
    parentRecordId: config && config.parentRecordId,
    relatedListId: config && config.relatedListId,
    fields: normalizeFields(config && config.fields),
    optionalFields: normalizeFields(config && config.optionalFields),
    sortBy: normalizeFields(config && config.sortBy),
    pageSize: config && config.pageSize,
    pageToken: config && config.pageToken,
    where: config && config.where
  });
});
export const getRelatedListCount = createFetchWireAdapter("/lightning/wire/getRelatedListCount", (config) => {
  if (!recordConfig(config) || !config.relatedListId) return null;
  return compactBody({
    parentRecordId: config.parentRecordId,
    relatedListId: config.relatedListId,
    maxCount: typeof config.maxCount === "number" ? config.maxCount : undefined
  });
});
export const getRelatedListInfo = createFetchWireAdapter("/lightning/wire/getRelatedListInfo", (config) => {
  if (!objectConfig(config) || !config.relatedListId) return null;
  return compactBody({ ...config, fields: normalizeFields(config.fields), optionalFields: normalizeFields(config.optionalFields) });
});
export const getRelatedListsInfo = createFetchWireAdapter("/lightning/wire/getRelatedListsInfo", (config) => {
  return objectConfig(config) ? compactBody(config) : null;
});
export const getRelatedListRecordsBatch = createFetchWireAdapter("/lightning/wire/getRelatedListRecordsBatch", (config) => {
  if (!recordConfig(config) || !Array.isArray(config.relatedListParameters)) return null;
  return compactBody(config);
});
export const getRelatedListInfoBatch = createFetchWireAdapter("/lightning/wire/getRelatedListInfoBatch", (config) => {
  if (!objectConfig(config) || !Array.isArray(config.relatedListNames)) return null;
  return compactBody(config);
});
function recordConfig(config) {
  return config && typeof config.parentRecordId === "string" && /^[a-zA-Z0-9]{15}(?:[a-zA-Z0-9]{3})?$/.test(config.parentRecordId);
}
function objectConfig(config) {
  return config && typeof config.parentObjectApiName === "string" && config.parentObjectApiName.length > 0;
}
function normalizeFields(fields) {
  return (fields || []).map((field) => {
    if (field && typeof field === "object") {
      return field.objectApiName && field.fieldApiName ? field.objectApiName + "." + field.fieldApiName : field.fieldApiName;
    }
    return String(field);
  });
}
function compactBody(body) {
  const out = {};
  for (const [key, value] of Object.entries(body)) {
    if (value !== undefined) {
      out[key] = value;
    }
  }
  return out;
}
`
}

func ShowToastEventModuleJS() string {
	return `import { recordToast } from "/lightning/runtime/shell/toast-service.js";
export { recordToast };
export const SHOW_TOAST_EVENT_NAME = "lightning__showtoast";
export class ShowToastEvent extends CustomEvent {
  constructor(detail = {}) {
    // Snapshot before super so a throwing option getter throws synchronously.
    // Shell listeners consume the captured public envelope without a private
    // channel that changes its delivered message or variant.
    const options = { ...detail };
    super("lightning__showtoast", {
      bubbles: true, composed: true, cancelable: true,
      detail: { label: options.title ?? "" },
    });
  }
}
export default ShowToastEvent;
`
}

func PlatformResourceLoaderModuleJS() string {
	return `const resourceLoads = new WeakMap();
const resourceScripts = new Set();
let scriptErrorsBound = false;
function bindResourceScriptErrors() {
  if (scriptErrorsBound || typeof window === "undefined") return;
  scriptErrorsBound = true;
  window.addEventListener("error", event => {
    // Only execution errors from scripts loaded through this API cross this
    // boundary. Ordinary application errors and failed DOM loads are intact.
    if (!resourceScripts.has(event.filename)) return;
    event.preventDefault();
    event.stopImmediatePropagation();
    console.error("Script error.", null);
  }, true);
}
function findTrackedLoad(tag, url) {
  // A matching element from another loader is not evidence of completion.
  for (const el of document.querySelectorAll(tag)) {
    const load = resourceLoads.get(el);
    if (load && load.url === url) {
      // Native calls share only an in-flight promise. Once settled, a new
      // promise retains the result without starting another resource load.
      if (load.state === "fulfilled") return Promise.resolve();
      if (load.state === "rejected") return Promise.reject(load.error);
      return load.promise;
    }
  }
}
function appendOnce(tag, attr, url) {
  // Resolve relative, empty and converted inputs before assigning the DOM
  // property. In particular, an empty link href otherwise never settles.
  url = new URL(String(url), document.baseURI).href;
  const existing = findTrackedLoad(tag, url);
  if (existing) {
    return Promise.resolve(existing);
  }
  const load = { url, state: "pending", promise: null, error: null };
  const el = document.createElement(tag);
  if (tag === "script") {
    bindResourceScriptErrors();
    resourceScripts.add(url);
  }
  load.promise = new Promise((resolve, reject) => {
    if (tag === "link") el.rel = "stylesheet";
    el[attr] = url;
    el.onload = () => {
      if (load.state !== "pending") return;
      load.state = "fulfilled";
      resolve();
    };
    el.onerror = () => {
      if (load.state !== "pending") return;
      load.state = "rejected";
      load.error = new Error("lightning/platformResourceLoader encountered an error loading '" + url + "'.");
      if (tag === "script") {
        // Captured sources have inner scheme http and path /not-found.
        // Map their origin to the local document without losing that path.
        const unavailableSource = new URL("/not-found", document.baseURI);
        unavailableSource.protocol = "http:";
        // Failed native script requests reach an unavailable blob before
        // rejecting. Let the browser report that failure, without replacing
        // or fabricating its HTTP diagnostic. Styles use their original URL.
        // A consumed script element does not start a second request when its
        // src changes. The captured blob request needs a fresh element.
        const unavailableScript = document.createElement("script");
        unavailableScript.src = "blob:" + unavailableSource.href;
        document.head.appendChild(unavailableScript);
      }
      reject(load.error);
    };
    document.head.appendChild(el);
  });
  // Retain settled failures as well as successes while the element exists.
  resourceLoads.set(el, load);
  return load.promise;
}
export function loadScript(_self, url) {
  // The native script URL conversion throws synchronously for null/undefined;
  // the built-in conversion also preserves number and object string inputs.
  const converted = String.prototype.split.call(url, undefined)[0];
  return appendOnce("script", "src", converted);
}
export function loadStyle(_self, url) {
  return appendOnce("link", "href", url);
}
`
}

func MessageServiceModuleJS() string {
	return `export {
  APPLICATION_SCOPE,
  MessageContext,
  createMessageContext,
  releaseMessageContext,
  subscribe,
  unsubscribe,
  publish,
} from "/lightning/runtime/shell/message-service.js";
`
}

func ResolveLabelValue(org *storage.OrgState, qualified string) (string, bool) {
	namespace, name, ok := splitNamespaceQualified(qualified)
	if !ok {
		return "", false
	}
	orgNamespace := ""
	if org != nil {
		orgNamespace = org.Namespace
	}
	var registry storage.MetadataRegistry
	if org != nil {
		registry = org.Metadata
	}
	value, status := resource.ResolveLabel(registry, orgNamespace, namespace, name)
	switch status {
	case resource.LabelLookupResolved, resource.LabelLookupPlatformFallback, resource.LabelLookupManagedNamespaceFallback:
		return value, true
	default:
		return "", false
	}
}

func ParseSchemaFieldToken(qualified string) (objectName, fieldName string, ok bool) {
	qualified = strings.TrimSpace(qualified)
	dot := strings.LastIndex(qualified, ".")
	if dot <= 0 || dot >= len(qualified)-1 {
		return "", "", false
	}
	return qualified[:dot], qualified[dot+1:], true
}

func ParseSchemaObjectToken(qualified string) (objectName string, ok bool) {
	qualified = strings.TrimSpace(qualified)
	if qualified == "" || strings.Contains(qualified, ".") || strings.Contains(qualified, "/") {
		return "", false
	}
	return qualified, true
}

func ParseApexWireToken(qualified string) (className, methodName string, ok bool) {
	qualified = strings.TrimSpace(qualified)
	dot := strings.LastIndex(qualified, ".")
	if dot <= 0 || dot >= len(qualified)-1 {
		return "", "", false
	}
	return qualified[:dot], qualified[dot+1:], true
}

func defaultExportJS(value any) string {
	raw, err := json.Marshal(value)
	if err != nil {
		raw = []byte("null")
	}
	return "export default " + string(raw) + ";\n"
}

func unsupportedModuleJS(message string) string {
	raw, err := json.Marshal(message)
	if err != nil {
		raw = []byte(`"Unsupported Salesforce module"`)
	}
	return "throw new Error(" + string(raw) + ");\nexport default undefined;\n"
}

func splitNamespaceQualified(qualified string) (namespace, name string, ok bool) {
	qualified = strings.TrimSpace(qualified)
	dot := strings.LastIndex(qualified, ".")
	if dot <= 0 || dot >= len(qualified)-1 {
		return "", "", false
	}
	return qualified[:dot], qualified[dot+1:], true
}
