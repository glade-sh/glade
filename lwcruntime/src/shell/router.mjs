import { navigateToUrl } from "./navigation-service.mjs";

export function currentPath() {
  return window.location.pathname + window.location.search;
}

export function routeKind(path = currentPath()) {
  if (path.includes("glade__unavailablePageReference=")) return "unavailable";
  if (path.includes("/lwc/preview/record/")) return "record";
  if (path.includes("/lwc/preview/action/")) return "action";
  if (path.includes("/lwc/preview/cmp/")) return "component";
  if (path.includes("/lwc/preview/component/")) return "component";
  if (path.includes("/lwc/preview/flow/")) return "flow";
  if (path.includes("/lwc/preview/app/")) return "app";
  if (path.includes("/lwc/preview/home")) return "home";
  if (path.includes("/lwc/preview/tab/")) return "tab";
  return "workbench";
}

export function navigateTo(url) {
  navigateToUrl(url);
}

export function bindRouteLinks(root = document) {
  root.addEventListener("click", (event) => {
    const link = event.target.closest?.("[data-glade-route]");
    if (!link) {
      return;
    }
    event.preventDefault();
    navigateTo(link.getAttribute("href") || link.dataset.gladeRoute);
  });
}
