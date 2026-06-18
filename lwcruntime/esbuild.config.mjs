import esbuild from "esbuild";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const srcDir = path.join(__dirname, "src");
const lightningEntryPoints = fs.readdirSync(path.join(srcDir, "lightning"))
  .filter((name) => name.endsWith(".mjs") && name !== "base.mjs")
  .map((name) => path.join(srcDir, "lightning", name));

await esbuild.build({
  entryPoints: [path.join(srcDir, "glade.out.mjs")],
  bundle: false,
  format: "esm",
  outfile: path.join(__dirname, "../internal/lwcruntime/embed/glade.out.js"),
  platform: "browser",
});

await esbuild.build({
  entryPoints: [
    path.join(srcDir, "shell/app.mjs"),
    path.join(srcDir, "shell/community-host.mjs"),
    path.join(srcDir, "shell/router.mjs"),
    path.join(srcDir, "shell/context-panel.mjs"),
    path.join(srcDir, "shell/diagnostics.mjs"),
    path.join(srcDir, "slds/slds-loader.mjs"),
    path.join(srcDir, "shims/community.mjs"),
    path.join(srcDir, "shims/site.mjs"),
    ...lightningEntryPoints,
  ],
  bundle: false,
  format: "esm",
  outdir: path.join(__dirname, ".esbuild-check"),
  platform: "browser",
  write: false,
});

console.log("built internal/lwcruntime/embed/glade.out.js");
