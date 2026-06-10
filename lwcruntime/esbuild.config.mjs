import esbuild from "esbuild";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));

await esbuild.build({
  entryPoints: [path.join(__dirname, "src/glade.out.mjs")],
  bundle: false,
  format: "esm",
  outfile: path.join(__dirname, "../internal/lwcruntime/embed/glade.out.js"),
  platform: "browser",
});

console.log("built internal/lwcruntime/embed/glade.out.js");
