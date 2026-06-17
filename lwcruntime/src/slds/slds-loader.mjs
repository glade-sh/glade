import { reportDiagnostic } from "@glade/shell/diagnostics";

const THEMES = {
  slds1: "glade-slds.css",
  slds2: "glade-slds.css",
};

export function sldsHref({ theme = "slds2", basePath = "/lightning/runtime/slds" } = {}) {
  const file = THEMES[theme];
  if (!file) {
    return "";
  }
  return `${basePath.replace(/\/$/, "")}/${file}`;
}

export function loadSLDS(options = {}) {
  const theme = options.theme || "slds2";
  const href = options.href || sldsHref({ theme, basePath: options.basePath });
  if (!href) {
    reportDiagnostic({
      code: "GLADELWC062",
      severity: "warning",
      message: `GLADELWC062 SLDS asset missing: ${theme}`,
      asset: theme,
    });
    return Promise.resolve({ ok: false, href: "", theme });
  }
  const existing = document.querySelector(`link[data-glade-slds][href="${href}"]`);
  if (existing) {
    return Promise.resolve({ ok: true, href, theme });
  }
  return new Promise((resolve) => {
    const link = document.createElement("link");
    link.rel = "stylesheet";
    link.href = href;
    link.dataset.gladeSlds = theme;
    link.onload = () => resolve({ ok: true, href, theme });
    link.onerror = () => {
      reportDiagnostic({
        code: "GLADELWC062",
        severity: "warning",
        message: `GLADELWC062 SLDS asset missing: ${href}`,
        asset: href,
      });
      resolve({ ok: false, href, theme });
    };
    document.head.appendChild(link);
  });
}

export default loadSLDS;
