import { loadSLDS } from "@glade/slds";
import { bindRouteLinks, routeKind } from "./router.mjs";
import { diagnostics } from "./diagnostics.mjs";
import { renderContextPanel } from "./context-panel.mjs";
import { installToastService } from "./toast-service.mjs";

export async function bootGladeShell({ root = document.body, config = readConfig() } = {}) {
  await loadSLDS();
  document.documentElement.dataset.gladeShell = routeKind();
  bindRouteLinks(root);
  const disposeToastService = installToastService(root);
  const panel = root.querySelector("[data-glade-context-panel]");
  if (panel) {
    renderContextPanel(panel, config);
  }
  document.addEventListener("glade:diagnostic", () => {
    if (panel) {
      renderContextPanel(panel, config);
    }
  });
  return { config, diagnostics, disposeToastService };
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
