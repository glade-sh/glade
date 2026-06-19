const diagnosticCode = "GLADELWC095";

function reportDiagnostic(message) {
  if (typeof window === "undefined") {
    return null;
  }
  const diagnostics = window.__gladeDiagnostics || [];
  window.__gladeDiagnostics = diagnostics;
  const entry = {
    code: diagnosticCode,
    severity: "warning",
    message,
  };
  diagnostics.push(entry);
  if (typeof document !== "undefined" && typeof document.dispatchEvent === "function" && typeof CustomEvent === "function") {
    document.dispatchEvent(new CustomEvent("glade:diagnostic", { detail: entry }));
  }
  return entry;
}

export function createUnavailableError(operation = "uiLearningPlatformApi") {
  const body = {
    errorCode: diagnosticCode,
    message: `${operation} is hosted-only and is not available in local Glade LWC preview.`,
  };
  const err = new Error(body.message);
  err.body = body;
  err.status = 501;
  return err;
}

export function hostedOnly(operation = "uiLearningPlatformApi") {
  const err = createUnavailableError(operation);
  reportDiagnostic(err.body.message);
  return Promise.reject(err);
}

export function getLearningItem() {
  return hostedOnly("getLearningItem");
}

export function getLearningItems() {
  return hostedOnly("getLearningItems");
}

export function getLearningItemProgress() {
  return hostedOnly("getLearningItemProgress");
}

export function getLearningProgram() {
  return hostedOnly("getLearningProgram");
}

export function getLearningPrograms() {
  return hostedOnly("getLearningPrograms");
}

export default {
  createUnavailableError,
  getLearningItem,
  getLearningItemProgress,
  getLearningItems,
  getLearningProgram,
  getLearningPrograms,
  hostedOnly,
};
