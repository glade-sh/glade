import { loadSLDS } from "@glade/slds";
import { bindRouteLinks, routeKind } from "./router.mjs";
import { diagnostics } from "./diagnostics.mjs";
import { renderContextPanel } from "./context-panel.mjs";
import { installToastService } from "./toast-service.mjs";
import { applyCommunityHost } from "./community-host.mjs";
import { bootWorkbenchBuilder } from "./workbench-builder.mjs";

export async function bootGladeShell({ root = document.body, config = readConfig() } = {}) {
  await loadSLDS();
  document.documentElement.dataset.gladeShell = routeKind();
  applyCommunityHost(root);
  bindRouteLinks(root);
  const disposeToastService = installToastService(root);
  const panel = root.querySelector("[data-glade-context-panel]");
  const renderPanel = () => {
    if (panel) {
      renderContextPanel(panel, readConfig());
    }
  };
  renderPanel();
  document.addEventListener("glade:diagnostic", renderPanel);
  document.addEventListener("glade:page-reference", renderPanel);
  document.addEventListener("glade:context-changed", renderPanel);
  bindFlowDiagnostics(root);
  bootWorkbenchBuilder(root, config);
  return { config, diagnostics, disposeToastService };
}

function bindFlowDiagnostics(root) {
  for (const eventName of ["flowattributechange", "flownavigationnext", "flownavigationback", "flownavigationpause", "flownavigationfinish"]) {
    root.addEventListener(eventName, captureFlowEvent);
  }
}

function captureFlowEvent(event) {
  document.dispatchEvent(new CustomEvent("glade:flow-event", {
    detail: { type: event.type, detail: event.detail || {} },
  }));
}

export function readConfig() {
  const node = document.getElementById("glade-lightning-config");
  if (!node) {
    return {};
  }
  try {
    return JSON.parse(node.textContent || "{}");
  } catch (_err) {
    return {};
  }
}

if (document.readyState === "loading") {
  document.addEventListener("DOMContentLoaded", () => bootGladeShell());
} else {
  bootGladeShell();
}
