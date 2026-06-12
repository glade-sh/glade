#!/usr/bin/env node
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { transformSync } from "@lwc/compiler";

const __dirname = path.dirname(fileURLToPath(import.meta.url));

function readStdin() {
  return new Promise((resolve, reject) => {
    let data = "";
    process.stdin.setEncoding("utf8");
    process.stdin.on("data", (chunk) => {
      data += chunk;
    });
    process.stdin.on("end", () => {
      try {
        resolve(JSON.parse(data));
      } catch (err) {
        reject(err);
      }
    });
    process.stdin.on("error", reject);
  });
}

function kebabCase(name) {
  return name.replace(/([a-z])([A-Z])/g, "$1-$2").toLowerCase();
}

function compileBundle(bundleDir, bundleName, namespace, outDir) {
  const jsFile = path.join(bundleDir, `${bundleName}.js`);
  const htmlFile = path.join(bundleDir, `${bundleName}.html`);
  const cssFile = path.join(bundleDir, `${bundleName}.css`);
  if (!fs.existsSync(jsFile) || !fs.existsSync(htmlFile)) {
    return null;
  }
  let jsSource = fs.readFileSync(jsFile, "utf8");
  const htmlSource = fs.readFileSync(htmlFile, "utf8");
  const cssSource = fs.existsSync(cssFile) ? fs.readFileSync(cssFile, "utf8") : "";

  const templateResult = transformSync(htmlSource, {
    filename: `${bundleName}.html`,
    namespace,
    name: bundleName,
  });
  jsSource = jsSource + "\n" + templateResult.code;

  const cssResult = cssSource
    ? transformSync(cssSource, {
        filename: `${bundleName}.css`,
        namespace,
        name: bundleName,
      })
    : { code: "" };

  const jsResult = transformSync(jsSource, {
    filename: `${bundleName}.js`,
    namespace,
    name: bundleName,
  });

  const qualified = `${namespace}/${bundleName}`;
  const outFile = path.join(outDir, qualified + ".js");
  fs.mkdirSync(path.dirname(outFile), { recursive: true });

  const moduleCode =
    cssResult.code +
    "\n" +
    jsResult.code +
    "\n" +
    `export default ${bundleName};\n`;

  fs.writeFileSync(outFile, moduleCode, "utf8");
  return {
    qualified: `${namespace}:${bundleName}`,
    moduleKey: qualified,
    file: outFile,
    tag: `${namespace}-${kebabCase(bundleName)}`,
  };
}

async function main() {
  const config = await readStdin();
  const projectRoot = path.resolve(config.projectRoot || ".");
  const outDir = path.resolve(config.outDir);
  const namespace = (config.namespace || "c").trim() || "c";
  const modules = {};

  const lwcRoots = new Set();
  for (const rel of config.lwcFiles || []) {
    const abs = path.join(projectRoot, rel);
    lwcRoots.add(path.dirname(abs));
  }
  for (const rel of config.lwcHtmlFiles || []) {
    const abs = path.join(projectRoot, rel);
    lwcRoots.add(path.dirname(abs));
  }

  fs.mkdirSync(outDir, { recursive: true });
  for (const bundleDir of lwcRoots) {
    const bundleName = path.basename(bundleDir);
    const entry = compileBundle(bundleDir, bundleName, namespace, outDir);
    if (entry) {
      modules[entry.qualified] = {
        moduleKey: entry.moduleKey,
        tag: entry.tag,
        file: entry.file,
      };
    }
  }

  const manifestPath = path.join(outDir, "manifest.json");
  fs.writeFileSync(manifestPath, JSON.stringify({ modules }, null, 2), "utf8");
  process.stdout.write(JSON.stringify({ modules, manifestPath }));
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
