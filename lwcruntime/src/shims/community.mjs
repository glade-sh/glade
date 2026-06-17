import { reportDiagnostic } from "../shell/diagnostics.mjs";

const reportedDiagnostics = new Set();

export function readCommunityContext() {
  return readCommunityContextInternal({ report: true });
}

export function readCommunityContextQuiet() {
  return readCommunityContextInternal({ report: false });
}

function readCommunityContextInternal({ report } = { report: true }) {
  const node = document.getElementById("glade-lwc-context");
  if (!node) {
    if (report) {
      reportOnce("GLADELWC100", "community context required");
    }
    return reportMissingIds(defaultCommunityContext(), report);
  }
  try {
    const context = JSON.parse(node.textContent || "{}");
    const community = normalizeCommunityContext(context.community || {});
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
    basePath: stringValue(community.basePath, "/s"),
    siteId: stringValue(community.siteId, ""),
    networkId: stringValue(community.networkId, ""),
    guest: Boolean(community.guest),
    language: stringValue(community.language, ""),
  };
}

function defaultCommunityContext() {
  return normalizeCommunityContext({});
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
