export interface PreviewRoute {
  label: string;
  path: string;
  sourcePath?: string;
}

export interface PreviewServer {
  kind: "lwc" | "visualforce";
  url: string;
  addr: string;
  running: true;
  routes: PreviewRoute[];
}

export interface ToolchainStatus {
  ok: boolean;
  path?: string;
  detail: string;
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
