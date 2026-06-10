#!/usr/bin/env node
import fs from "node:fs";
import path from "node:path";
import { transformSync } from "@lwc/compiler";

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

function writeCompiled(outFile, code) {
  fs.mkdirSync(path.dirname(outFile), { recursive: true });
  fs.writeFileSync(outFile, code, "utf8");
}

function rewriteStylesheetImports(code, bundleName) {
  return code
    .replace(
      `from "./${bundleName}.html"`,
      `from "./${bundleName}.html.js"`
    )
    .replace(
      `from "./${bundleName}.css"`,
      `from "./${bundleName}.css.js"`
    )
    .replace(
      `from "./${bundleName}.scoped.css?scoped=true"`,
      `from "./${bundleName}.scoped.css.js"`
    );
}

function compileBundle(bundleDir, bundleName, namespace, outDir) {
  const jsPath = path.join(bundleDir, `${bundleName}.js`);
  const htmlPath = path.join(bundleDir, `${bundleName}.html`);
  const cssPath = path.join(bundleDir, `${bundleName}.css`);
  if (!fs.existsSync(jsPath) || !fs.existsSync(htmlPath)) {
    return null;
  }

  const moduleKey = `${namespace}/${bundleName}`;
  const bundleOut = path.join(outDir, moduleKey);
  fs.mkdirSync(bundleOut, { recursive: true });

  const htmlSource = fs.readFileSync(htmlPath, "utf8");
  const htmlResult = transformSync(htmlSource, htmlPath, {
    namespace,
    name: bundleName,
  });
  writeCompiled(
    path.join(bundleOut, `${bundleName}.html.js`),
    rewriteStylesheetImports(htmlResult.code, bundleName)
  );

  if (fs.existsSync(cssPath)) {
    const cssSource = fs.readFileSync(cssPath, "utf8");
    const cssResult = transformSync(cssSource, cssPath, {
      namespace,
      name: bundleName,
    });
    writeCompiled(path.join(bundleOut, `${bundleName}.css.js`), cssResult.code);
    writeCompiled(
      path.join(bundleOut, `${bundleName}.scoped.css.js`),
      "export default '';\n"
    );
  } else {
    writeCompiled(path.join(bundleOut, `${bundleName}.css.js`), "export default '';\n");
    writeCompiled(
      path.join(bundleOut, `${bundleName}.scoped.css.js`),
      "export default '';\n"
    );
  }

  const jsSource = fs.readFileSync(jsPath, "utf8");
  const jsResult = transformSync(jsSource, jsPath, {
    namespace,
    name: bundleName,
  });
  const jsCode = rewriteStylesheetImports(jsResult.code, bundleName);

  const entryFile = path.join(bundleOut, `${bundleName}.js`);
  writeCompiled(entryFile, jsCode);
  compileSiblingModules(bundleDir, bundleName, namespace, bundleOut);

  return {
    qualified: `${namespace}:${bundleName}`,
    moduleKey,
    file: entryFile,
    tag: `${namespace}-${kebabCase(bundleName)}`,
  };
}

function compileSiblingModules(bundleDir, bundleName, namespace, bundleOut) {
  for (const entry of fs.readdirSync(bundleDir)) {
    if (!entry.endsWith(".js")) {
      continue;
    }
    if (entry === `${bundleName}.js`) {
      continue;
    }
    const jsPath = path.join(bundleDir, entry);
    if (!fs.statSync(jsPath).isFile()) {
      continue;
    }
    const siblingName = entry.slice(0, -3);
    const jsSource = fs.readFileSync(jsPath, "utf8");
    const jsResult = transformSync(jsSource, jsPath, {
      namespace,
      name: siblingName,
    });
    writeCompiled(
      path.join(bundleOut, entry),
      rewriteStylesheetImports(jsResult.code, siblingName)
    );
  }
}

async function main() {
  const config = await readStdin();
  const projectRoot = path.resolve(config.projectRoot || ".");
  const outDir = path.resolve(config.outDir);
  const namespace = (config.namespace || "c").trim() || "c";
  const modules = {};

  const lwcRoots = new Set();
  for (const rel of config.lwcFiles || []) {
    lwcRoots.add(path.dirname(path.join(projectRoot, rel)));
  }
  for (const rel of config.lwcHtmlFiles || []) {
    lwcRoots.add(path.dirname(path.join(projectRoot, rel)));
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
