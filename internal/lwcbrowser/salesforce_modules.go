package lwcbrowser

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/glade-sh/glade/internal/resource"
	"github.com/glade-sh/glade/internal/storage"
)

// SalesforceImportMap returns import-map entries for @salesforce/* and lightning/* modules.
func SalesforceImportMap() map[string]string {
	imports := map[string]string{
		"@glade/shell/app":                      "/lightning/runtime/shell/app.js",
		"@glade/shell/router":                   "/lightning/runtime/shell/router.js",
		"@glade/shell/contextPanel":             "/lightning/runtime/shell/context-panel.js",
		"@glade/shell/diagnostics":              "/lightning/runtime/shell/diagnostics.js",
		"@glade/slds":                           "/lightning/runtime/slds/slds-loader.js",
		"@salesforce/apex":                      "/lightning/shims/core/apex.js",
		"@salesforce/apex/":                     "/lightning/shims/apex/",
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
		"@salesforce/site/Id":                   "/lightning/shims/site/Id.js",
		"@salesforce/user/":                     "/lightning/shims/user/",
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
		"lightning/platformWorkspaceApi":        "/lightning/shims/lightning/platformWorkspaceApi.js",
		"lightning/prompt":                      "/lightning/shims/lightning/prompt.js",
		"lightning/purifyLib":                   "/lightning/shims/lightning/purifyLib.js",
		"lightning/refresh":                     "/lightning/shims/lightning/refresh.js",
		"lightning/routingService":              "/lightning/shims/lightning/routingService.js",
		"lightning/showToastEvent":              "/lightning/shims/lightning/showToastEvent.js",
		"lightning/toast":                       "/lightning/shims/lightning/toast.js",
		"lightning/uiLayoutApi":                 "/lightning/shims/lightning/uiLayoutApi.js",
		"lightning/uiListApi":                   "/lightning/shims/lightning/uiListApi.js",
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
	return `export default class LightningConfirm {
  static open(options = {}) {
    window.dispatchEvent(new CustomEvent("gladeconfirm", { detail: options, bubbles: true, composed: true }));
    return Promise.resolve(true);
  }
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
	return fmt.Sprintf(`export const permissionName = %q;
export default true;
`, strings.TrimSuffix(strings.TrimSpace(name), ".js"))
}

func ActionsModuleJS() string {
	return `export class CloseActionScreenEvent extends CustomEvent {
  constructor() {
    super("closeactionscreen", { bubbles: true, composed: true });
  }
}
`
}

func FlowSupportModuleJS() string {
	return `export class FlowAttributeChangeEvent extends CustomEvent {
  constructor(attributeName, value) {
    super("flowattributechange", { bubbles: true, composed: true, detail: { attributeName, value } });
  }
}
function flowNavigationEvent(type) {
  return class extends CustomEvent {
    constructor() {
      super(type, { bubbles: true, composed: true });
    }
  };
}
export const FlowNavigationNextEvent = flowNavigationEvent("flownavigationnext");
export const FlowNavigationBackEvent = flowNavigationEvent("flownavigationback");
export const FlowNavigationPauseEvent = flowNavigationEvent("flownavigationpause");
export const FlowNavigationFinishEvent = flowNavigationEvent("flownavigationfinish");
`
}

func RefreshModuleJS() string {
	return `const handlers = new Map();
const containers = new Map();
export class RefreshEvent extends CustomEvent {
  constructor() {
    super("lightning__refresh", { bubbles: true, composed: true });
  }
}
export function registerRefreshHandler(element, handler) {
  handlers.set(element, handler);
  return { element, handler };
}
export function unregisterRefreshHandler(element) {
  handlers.delete(element && element.element || element);
}
export function registerRefreshContainer(element, callback) {
  containers.set(element, callback);
  return { element, callback };
}
export function unregisterRefreshContainer(element) {
  containers.delete(element && element.element || element);
}
export async function __gladeDispatchRefresh(root) {
  const results = [];
  for (const [element, handler] of handlers) {
    if (!root || root === element || (root.contains && root.contains(element))) {
      results.push(await handler());
    }
  }
  return results;
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
		return defaultExportJS(userID)
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

func CommunityModuleJS(property string) string {
	switch property {
	case "basePath":
		return `import { readCommunityValue } from "/lightning/runtime/shims/community.js";
export default readCommunityValue("basePath", "/s");
`
	case "Id":
		return `import { readCommunityValue } from "/lightning/runtime/shims/community.js";
export default readCommunityValue("networkId", "");
`
	default:
		return unsupportedModuleJS("Unsupported @salesforce/community property: " + property)
	}
}

func SiteModuleJS(property string) string {
	switch property {
	case "Id":
		return `import { readSiteId } from "/lightning/runtime/shims/site.js";
export default readSiteId();
`
	default:
		return unsupportedModuleJS("Unsupported @salesforce/site property: " + property)
	}
}

func I18nModuleJS(property string) string {
	values := map[string]any{
		"currency":                  "USD",
		"dateTime.mediumDateFormat": "MMM d, yyyy",
		"dateTime.mediumTimeFormat": "h:mm:ss a",
		"dateTime.shortDateFormat":  "M/d/yyyy",
		"dir":                       "ltr",
		"firstDayOfWeek":            1,
		"isEasternNameStyle":        false,
		"lang":                      "en-US",
		"locale":                    "en-US",
		"number.currencyFormat":     "¤#,##0.00;(¤#,##0.00)",
		"number.currencySymbol":     "$",
		"number.decimalSeparator":   ".",
		"number.groupingSeparator":  ",",
		"number.numberFormat":       "#,##0.###",
		"number.percentFormat":      "#,##0%",
		"timeZone":                  "UTC",
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
  toString() { return %q; },
};
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
  generateUrl,
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
    [NavigationMixin.Navigate](pageReference) {
      navigate(pageReference).catch(() => undefined);
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
export {
  EnclosingTabId,
  IsConsoleNavigation,
  closeTab,
  configureWorkspace,
  disableTabClose,
  focusTab,
  getAllTabInfo,
  getFocusedTabInfo,
  getTabInfo,
  isConsoleNavigation,
  openSubtab,
  openTab,
  refreshTab,
  setTabHighlighted,
  setTabIcon,
  setTabLabel,
  workspaceDiagnosticCodes,
} from "/lightning/runtime/shell/workspace-service.js";
`
}

func UIRecordAPIModuleJS() string {
	return `import { createFetchWireAdapter, createGetRecordWireAdapter } from "/lightning/shims/core/wire-adapter.js";
import {
  getRecordNotifyChange,
  notifyRecordUpdateAvailable,
  refreshApex,
} from "/lightning/shims/core/lds-cache.mjs";
export { getRecordNotifyChange, notifyRecordUpdateAvailable, refreshApex };
export const getRecord = createGetRecordWireAdapter();
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
  if (!records.length) {
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
  const apiName = objectApiName(config && config.objectApiName);
  return apiName ? { objectApiName: apiName } : null;
});
export const getObjectInfos = createFetchWireAdapter("/lightning/wire/getObjectInfos", (config) => {
  const objectApiNames = (config && config.objectApiNames || []).map(objectApiName).filter(Boolean);
  return objectApiNames.length ? { objectApiNames } : null;
});
export const getRecordCreateDefaults = createFetchWireAdapter("/lightning/wire/getRecordCreateDefaults", (config) => {
  const apiName = objectApiName(config && config.objectApiName);
  if (!apiName) {
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
  const apiName = objectApiName(config && config.objectApiName);
  const fieldName = fieldApiName(config && config.fieldApiName);
  if (!fieldName || !(config && config.recordTypeId) || (!apiName && !fieldName.includes("."))) {
    return null;
  }
  return compactBody({
    objectApiName: apiName,
    fieldApiName: fieldName,
    recordTypeId: config && config.recordTypeId
  });
});
export const getPicklistValuesByRecordType = createFetchWireAdapter("/lightning/wire/getPicklistValuesByRecordType", (config) => {
  const apiName = objectApiName(config && config.objectApiName);
  if (!apiName || !(config && config.recordTypeId)) {
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
  }).then((data) => notifyRecordUpdateAvailable(notificationItems(data)).then(() => data));
}
export function updateRecord(recordInput) {
  const recordId = recordInput && recordInput.fields && recordInput.fields.Id;
  return post("/lightning/wire/updateRecord", {
    fields: recordInput && recordInput.fields || {}
  }).then((data) => notifyRecordUpdateAvailable(notificationItems(data, recordId)).then(() => data));
}
export function deleteRecord(recordId) {
  return post("/lightning/wire/deleteRecord", { recordId })
    .then((data) => notifyRecordUpdateAvailable(notificationItems(data, recordId)).then(() => data));
}
export async function __gladeRecordPickerSearch(config = {}) {
  return post("/lightning/wire/recordPickerSearch", config);
}
export function generateRecordInputForCreate(record, objectInfo) {
  const fields = recordFields(record, objectInfo, "createable");
  delete fields.Id;
  return {
    apiName: record && (record.apiName || record.objectApiName),
    fields
  };
}
export function generateRecordInputForUpdate(record, objectInfo) {
  const fields = recordFields(record, objectInfo, "updateable");
  const id = record && (record.id || record.recordId || fieldValue(record.fields && record.fields.Id));
  if (id !== undefined && id !== null) {
    fields.Id = id;
  }
  return { fields };
}
export function createRecordInputFilteredByEditedFields(recordInput, originalRecord) {
  const sourceFields = recordInput && recordInput.fields || {};
  const out = {};
  for (const [name, value] of Object.entries(sourceFields)) {
    if (name === "Id") {
      out[name] = value;
      continue;
    }
    const original = originalRecord && originalRecord.fields && originalRecord.fields[name];
    if (!sameValue(value, fieldValue(original))) {
      out[name] = value;
    }
  }
  return Object.assign({}, recordInput || {}, { fields: out });
}
export function getFieldValue(record, field) {
  const name = typeof field === "string" ? field.split(".").pop() : field && field.fieldApiName;
  const value = record && record.fields && record.fields[name];
  return value ? value.value : undefined;
}
export function getFieldDisplayValue(record, field) {
  const name = typeof field === "string" ? field.split(".").pop() : field && field.fieldApiName;
  const value = record && record.fields && record.fields[name];
  return value ? value.displayValue : undefined;
}
function recordFields(record, objectInfo, accessProperty) {
  const fields = {};
  const source = record && record.fields || {};
  for (const [name, wrapped] of Object.entries(source)) {
    if (name === "Id" && accessProperty === "createable") {
      continue;
    }
    if (!fieldAllows(objectInfo, name, accessProperty)) {
      continue;
    }
    const value = fieldValue(wrapped);
    if (!recordInputValueSupported(value)) {
      continue;
    }
    fields[name] = value;
  }
  return fields;
}
function fieldAllows(objectInfo, name, accessProperty) {
  const fields = objectInfo && objectInfo.fields || {};
  const field = fields[name];
  if (!field) {
    return true;
  }
  return field[accessProperty] !== false;
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
	return `export const getListUi = class GetListUiUnsupportedAdapter {
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
`
}

func UILayoutAPIModuleJS() string {
	return `import { createFetchWireAdapter } from "/lightning/shims/core/wire-adapter.js";
export const getLayout = createFetchWireAdapter("/lightning/wire/getLayout", (config) => {
  const apiName = objectApiName(config && config.objectApiName);
  if (!apiName) {
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
	return `import { createFetchWireAdapter } from "/lightning/shims/core/wire-adapter.js";
export const getObjectInfo = createFetchWireAdapter("/lightning/wire/getObjectInfo", (config) => {
  const apiName = objectApiName(config && config.objectApiName);
  return apiName ? { objectApiName: apiName } : null;
});
export const getObjectInfos = createFetchWireAdapter("/lightning/wire/getObjectInfos", (config) => {
  const objectApiNames = (config && config.objectApiNames || []).map(objectApiName).filter(Boolean);
  return objectApiNames.length ? { objectApiNames } : null;
});
export const getPicklistValues = createFetchWireAdapter("/lightning/wire/getPicklistValues", (config) => {
  const apiName = objectApiName(config && config.objectApiName);
  const fieldName = fieldApiName(config && config.fieldApiName);
  if (!fieldName || !(config && config.recordTypeId) || (!apiName && !fieldName.includes("."))) {
    return null;
  }
  return compactBody({
    objectApiName: apiName,
    fieldApiName: fieldName,
    recordTypeId: config && config.recordTypeId
  });
});
export const getPicklistValuesByRecordType = createFetchWireAdapter("/lightning/wire/getPicklistValuesByRecordType", (config) => {
  const apiName = objectApiName(config && config.objectApiName);
  if (!apiName || !(config && config.recordTypeId)) {
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

func UIRelatedListAPIModuleJS() string {
	return `import { createFetchWireAdapter } from "/lightning/shims/core/wire-adapter.js";
export const getRelatedListRecords = createFetchWireAdapter("/lightning/wire/getRelatedListRecords", (config) => {
  if (!(config && config.parentRecordId) || !config.relatedListId) {
    return null;
  }
  return compactBody({
    parentRecordId: config && config.parentRecordId,
    relatedListId: config && config.relatedListId,
    fields: normalizeFields(config && config.fields),
    optionalFields: normalizeFields(config && config.optionalFields),
    sortBy: normalizeFields(config && config.sortBy),
    pageSize: config && config.pageSize,
    pageToken: config && config.pageToken
  });
});
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
    super("lightning__showtoast", { bubbles: true, composed: true, cancelable: true, detail });
  }
}
export default ShowToastEvent;
`
}

func PlatformResourceLoaderModuleJS() string {
	return `function appendOnce(tag, attr, url) {
  if (!url) {
    return Promise.reject(new Error("resource URL is required"));
  }
  const selector = tag + "[" + attr + "=\"" + url + "\"]";
  if (document.querySelector(selector)) {
    return Promise.resolve();
  }
  return new Promise((resolve, reject) => {
    const el = document.createElement(tag);
    el[attr] = url;
    el.onload = () => resolve();
    el.onerror = () => reject(new Error("failed to load resource: " + url));
    document.head.appendChild(el);
  });
}
export function loadScript(_self, url) {
  return appendOnce("script", "src", url);
}
export function loadStyle(_self, url) {
  const selector = "link[href=\"" + url + "\"]";
  if (document.querySelector(selector)) {
    return Promise.resolve();
  }
  return new Promise((resolve, reject) => {
    const el = document.createElement("link");
    el.rel = "stylesheet";
    el.href = url;
    el.onload = () => resolve();
    el.onerror = () => reject(new Error("failed to load resource: " + url));
    document.head.appendChild(el);
  });
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
