import { reportDiagnostic } from "../shell/diagnostics.mjs";

const reportedDiagnostics = new Set();

export function readCommunityContext() {
  return readCommunityContextInternal({ report: true });
}

export function readCommunityContextQuiet() {
  return readCommunityContextInternal({ report: false });
}

function readCommunityContextInternal({ report } = { report: true }) {
  if (typeof document === "undefined") {
    return defaultCommunityContext();
  }
  const node = document.getElementById("glade-lwc-context");
  if (!node) {
    if (report) {
      reportOnce("GLADELWC100", "community context required");
    }
    return reportMissingIds(defaultCommunityContext(), report);
  }
  try {
    const community = normalizeCommunityContext(readCommunityShell());
    if (!community.site && report) {
      reportOnce("GLADELWC100", "community context required");
    }
    return reportMissingIds(community, report);
  } catch (_err) {
    if (report) {
      reportOnce("GLADELWC100", "community context required");
    }
    return reportMissingIds(defaultCommunityContext(), report);
  }
}

export function readCommunityValue(name, fallback = "") {
  const context = readCommunityContext();
  const value = context?.[name];
  if (value === undefined || value === null || value === "") {
    return fallback;
  }
  return value;
}

export function normalizeCommunityContext(community = {}) {
  return {
    site: stringValue(community.site, ""),
    name: stringValue(community.name || community.communityName, ""),
    url: stringValue(community.url || community.communityUrl, ""),
    basePath: stringValue(community.basePath, "/s"),
    siteId: stringValue(community.siteId, ""),
    networkId: stringValue(community.networkId, ""),
    guest: Boolean(community.guest),
    language: stringValue(community.language, ""),
    activeLanguages: normalizeActiveLanguages(community.activeLanguages || community.languages),
    lwr: Boolean(community.lwr || community.isLwr),
    aura: Boolean(community.aura || community.isAura),
    routeParams: { ...(community.routeParams || {}) },
    menus: { ...(community.menus || {}) },
    managedContent: { ...(community.managedContent || {}) },
  };
}

function defaultCommunityContext() {
  return normalizeCommunityContext({});
}

function readCommunityShell() {
  const node = document.getElementById("glade-lwc-context");
  if (!node) {
    return {};
  }
  try {
    return JSON.parse(node.textContent || "{}").community || {};
  } catch (_err) {
    return {};
  }
}

function reportMissingIds(context, report = true) {
  if (report && (!context.siteId || !context.networkId)) {
    reportOnce("GLADELWC102", "community siteId or networkId is missing; local shims export empty IDs");
  }
  return context;
}

function reportOnce(code, message) {
  if (reportedDiagnostics.has(code)) {
    return;
  }
  reportedDiagnostics.add(code);
  reportDiagnostic({ code, severity: "warning", message });
}

function stringValue(value, fallback) {
  if (value === undefined || value === null || value === "") {
    return fallback;
  }
  return String(value);
}

function normalizeActiveLanguages(value) {
  if (!Array.isArray(value) || value.length === 0) {
    return [];
  }
  return value.map((language) => {
    if (typeof language === "string") {
      return { code: language, label: language, active: true };
    }
    const code = stringValue(language?.code || language?.language || language?.locale, "en-US");
    return {
      code,
      label: stringValue(language?.label || language?.name || code, code),
      active: language?.active === undefined ? true : Boolean(language.active),
    };
  });
}
