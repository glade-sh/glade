import esbuild from "esbuild";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const srcDir = path.join(__dirname, "src");

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
    path.join(srcDir, "shell/router.mjs"),
    path.join(srcDir, "shell/context-panel.mjs"),
    path.join(srcDir, "shell/diagnostics.mjs"),
    path.join(srcDir, "slds/slds-loader.mjs"),
    path.join(srcDir, "lightning/button.mjs"),
    path.join(srcDir, "lightning/buttonIcon.mjs"),
    path.join(srcDir, "lightning/card.mjs"),
    path.join(srcDir, "lightning/input.mjs"),
    path.join(srcDir, "lightning/textarea.mjs"),
    path.join(srcDir, "lightning/combobox.mjs"),
    path.join(srcDir, "lightning/layout.mjs"),
    path.join(srcDir, "lightning/layoutItem.mjs"),
    path.join(srcDir, "lightning/tabset.mjs"),
    path.join(srcDir, "lightning/tab.mjs"),
    path.join(srcDir, "lightning/spinner.mjs"),
    path.join(srcDir, "lightning/icon.mjs"),
    path.join(srcDir, "lightning/datatable.mjs"),
    path.join(srcDir, "lightning/recordForm.mjs"),
    path.join(srcDir, "lightning/recordViewForm.mjs"),
    path.join(srcDir, "lightning/recordEditForm.mjs"),
    path.join(srcDir, "lightning/outputField.mjs"),
    path.join(srcDir, "lightning/inputField.mjs"),
    path.join(srcDir, "lightning/messages.mjs"),
    path.join(srcDir, "lightning/modal.mjs"),
  ],
  bundle: false,
  format: "esm",
  outdir: path.join(__dirname, ".esbuild-check"),
  platform: "browser",
  write: false,
});

console.log("built internal/lwcruntime/embed/glade.out.js");
