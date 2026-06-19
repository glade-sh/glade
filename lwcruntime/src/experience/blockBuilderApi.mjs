const diagnosticCode = "GLADELWC093";

function readContext() {
  if (typeof document === "undefined") {
    return {};
  }
  const node = document.getElementById("glade-lwc-context");
  if (!node) {
    return {};
  }
  try {
    return JSON.parse(node.textContent || "{}");
  } catch (_err) {
    return {};
  }
}

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

function selectedBlock() {
  const block = readContext().experience?.selectedBlock;
  return block && typeof block === "object" ? block : undefined;
}

function unavailable(operation) {
  const body = {
    errorCode: diagnosticCode,
    message: `${operation} is hosted-only and is not available in local Glade LWC preview.`,
  };
  const err = new Error(body.message);
  err.body = body;
  err.status = 501;
  reportDiagnostic(body.message);
  return err;
}

export function getCurrentSelectedBlock(dataCallback) {
  if (typeof dataCallback === "function") {
    this.dataCallback = dataCallback;
    return;
  }
  return Promise.resolve({ data: selectedBlock(), error: undefined });
}

getCurrentSelectedBlock.prototype.connect = function connect() {
  if (typeof this.dataCallback === "function") {
    this.dataCallback({ data: selectedBlock(), error: undefined });
  }
};
getCurrentSelectedBlock.prototype.disconnect = function disconnect() {};
getCurrentSelectedBlock.prototype.update = getCurrentSelectedBlock.prototype.connect;

export function replaceBlock() {
  return Promise.reject(unavailable("replaceBlock"));
}

export default {
  getCurrentSelectedBlock,
  replaceBlock,
};
