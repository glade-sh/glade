// Serve the product's module bodies so record forms fixtures share its actual imports,
// wire adapters and mutations instead of maintaining partial API mocks.
import fs from "node:fs";
import path from "node:path";
import { repoRoot } from "./helpers.mjs";

const source = fs.readFileSync(path.join(repoRoot, "internal/lwcbrowser/salesforce_modules.go"), "utf8").replace(/\r/g, "");
function productModule(name) {
  const match = source.match(new RegExp("func " + name + "\\(\\) string \\{\\n\\treturn (objectMetadataConfigJS\\(\\) \\+ )?`([\\s\\S]*?)`\\n\\}"));
  if (match === null) throw new Error("Missing product module body: " + name);
  // UI API modules prepend the product's shared metadata validators.
  return (match[1] ? productModule("objectMetadataConfigJS") : "") + match[2];
}

const modules = new Map([
  ["/lightning/shims/lightning/uiRecordApi.js", productModule("UIRecordAPIModuleJS")],
  ["/lightning/shims/lightning/uiObjectInfoApi.js", productModule("UIObjectInfoAPIModuleJS")],
]);
const i18n = source.slice(source.indexOf("func I18nModuleJS("), source.indexOf("func SchemaFieldModuleJS("));
for (const name of ["locale", "currency"]) {
  const value = i18n.match(new RegExp('"' + name + '":\\s*("[^\\"]*")'))?.[1];
  if (value === undefined) throw new Error("Missing product i18n default: " + name);
  modules.set("/lightning/shims/i18n/" + name + ".js", "export default " + value + ";\n");
}

export const l19RuntimeImports = {
  "lightning/uiRecordApi": "/lightning/shims/lightning/uiRecordApi.js",
  "lightning/uiObjectInfoApi": "/lightning/shims/lightning/uiObjectInfoApi.js",
  "@salesforce/i18n/locale": "/lightning/shims/i18n/locale.js",
  "@salesforce/i18n/currency": "/lightning/shims/i18n/currency.js",
};

export function serveL19RuntimeModule(urlPath, res) {
  let body = modules.get(urlPath);
  if (urlPath === "/lightning/shims/core/wire-adapter.js") {
    body = fs.readFileSync(path.join(repoRoot, "lwcruntime/src/shims/wire-adapter.mjs"));
  } else if (urlPath === "/lightning/shims/core/lds-cache.mjs") {
    body = fs.readFileSync(path.join(repoRoot, "lwcruntime/src/shims/lds-cache.mjs"));
  }
  if (body === undefined) return false;
  res.writeHead(200, { "Content-Type": "application/javascript; charset=utf-8" });
  res.end(body);
  return true;
}
