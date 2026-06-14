export interface PreviewRoute {
  label: string;
  path: string;
  sourcePath?: string;
}

export interface PreviewServer {
  kind: "lwc" | "visualforce";
  url: string;
  addr: string;
  running: boolean;
  routes: PreviewRoute[];
}

export interface ToolchainStatus {
  ok: boolean;
  path?: string;
  detail: string;
}

export interface PreviewProcessFailure {
  code?: number | null;
  signal?: string | null;
  stdout?: string;
  stderr?: string;
}

interface LWCReadyFile {
  url: string;
  addr: string;
  routes: string[];
}

interface VFReadyFile {
  url: string;
  addr: string;
  pages: string[];
}

interface ToolchainStatusFile {
  ok: boolean;
  path?: string;
  detail?: string;
}

export function parseLWCReadyFile(raw: string): PreviewServer {
  const ready = JSON.parse(raw) as LWCReadyFile;
  return {
    kind: "lwc",
    url: ready.url,
    addr: ready.addr,
    running: true,
    routes: ready.routes.map(parseLWCRoute),
  };
}

export function parseVFReadyFile(raw: string): PreviewServer {
  const ready = JSON.parse(raw) as VFReadyFile;
  return {
    kind: "visualforce",
    url: ready.url,
    addr: ready.addr,
    running: true,
    routes: ready.pages.map((page) => ({
      label: lastPathPart(page),
      path: page,
    })),
  };
}

export function parseToolchainStatus(raw: string): ToolchainStatus {
  const status = JSON.parse(raw) as ToolchainStatusFile;
  return {
    ok: status.ok,
    ...(status.path ? { path: status.path } : {}),
    detail: status.detail || "unknown",
  };
}

export function stoppedPreviewServer(server: PreviewServer): PreviewServer {
  return { ...server, running: false };
}

export function formatPreviewStartFailure(reason: string, detail: PreviewProcessFailure = {}): string {
  const status = processStatus(detail);
  const output = firstUsefulOutput(detail.stderr) || firstUsefulOutput(detail.stdout);
  const prefix = status ? `${reason} (${status})` : reason;
  if (!output) {
    return prefix;
  }
  return `${prefix}: ${output}`;
}

function parseLWCRoute(route: string): PreviewRoute {
  const arrow = " -> ";
  const arrowIndex = route.indexOf(arrow);
  if (arrowIndex >= 0) {
    const sourcePath = route.slice(0, arrowIndex).trim();
    const path = route.slice(arrowIndex + arrow.length).trim();
    return {
      label: lastPathPart(path),
      path,
      sourcePath,
    };
  }

  return {
    label: lwcComponentLabel(route),
    path: route,
  };
}

function lwcComponentLabel(route: string): string {
  const prefix = "/lwc/preview/component/";
  if (route.startsWith(prefix)) {
    return route.slice(prefix.length);
  }
  return lastPathPart(route);
}

function lastPathPart(path: string): string {
  const trimmed = path.replace(/\/+$/, "");
  const slashIndex = trimmed.lastIndexOf("/");
  if (slashIndex < 0) {
    return trimmed;
  }
  return trimmed.slice(slashIndex + 1);
}

function processStatus(detail: PreviewProcessFailure): string {
  if (detail.code !== undefined && detail.code !== null) {
    return `exit code ${detail.code}`;
  }
  if (detail.signal) {
    return `signal ${detail.signal}`;
  }
  return "";
}

function firstUsefulOutput(value: string | undefined): string {
  const cleaned = (value || "")
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean)
    .join(" ");
  if (cleaned.length <= 500) {
    return cleaned;
  }
  return `${cleaned.slice(0, 497)}...`;
}
