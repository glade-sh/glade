export const diagnostics = window.__gladeDiagnostics || [];
window.__gladeDiagnostics = diagnostics;

export function reportDiagnostic(diagnostic) {
  const entry = {
    code: diagnostic?.code || "GLADELWC000",
    severity: diagnostic?.severity || "warning",
    message: diagnostic?.message || "Glade LWC diagnostic",
    ...diagnostic,
  };
  diagnostics.push(entry);
  document.dispatchEvent(new CustomEvent("glade:diagnostic", { detail: entry }));
  return entry;
}

export function clearDiagnostics() {
  diagnostics.splice(0, diagnostics.length);
}

export function diagnosticsByCode(code) {
  return diagnostics.filter((entry) => entry.code === code);
}
