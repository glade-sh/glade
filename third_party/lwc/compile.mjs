#!/usr/bin/env node
import fs from "node:fs";
import path from "node:path";
import { createRequire } from "node:module";
import { fileURLToPath } from "node:url";

const defaultToolchainDir = path.dirname(fileURLToPath(import.meta.url));
const toolchainDir = process.env.GLADE_LWC_TOOLCHAIN_DIR || defaultToolchainDir;
const requireFromToolchain = createRequire(path.join(toolchainDir, "compile.mjs"));
const { transformSync } = requireFromToolchain("@lwc/compiler");
const { parse } = requireFromToolchain("@babel/parser");

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

function transformOptions(namespace, name, apiVersion) {
  return {
    namespace,
    name,
    apiVersion,
    enableLwcOn: true,
    experimentalComplexExpressions: apiVersion >= 66,
  };
}

function transformTemplate(source, filename, namespace, name, apiVersion) {
  const result = transformSync(source, filename, transformOptions(namespace, name, apiVersion));
  const unsupportedDetailsName = result.warnings?.find(
    (warning) => warning.code === 1057 && warning.message.includes("name is not valid attribute for details")
  );
  if (apiVersion < 67 && unsupportedDetailsName) {
    throw new Error(unsupportedDetailsName.message);
  }
  return result;
}

function rewriteStylesheetImports(code, bundleName) {
  return rewriteTemplateRelativeImports(code)
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

function compileBundle(bundleDir, bundleName, namespace, outDir, apiVersion, moduleAvailability) {
  const jsPath = path.join(bundleDir, `${bundleName}.js`);
  const htmlPath = path.join(bundleDir, `${bundleName}.html`);
  const cssPath = path.join(bundleDir, `${bundleName}.css`);
  if (!fs.existsSync(jsPath)) {
    return null;
  }
  preflightBundleImports(bundleDir, apiVersion, moduleAvailability);
  if (!fs.existsSync(htmlPath) && !isCustomRenderComponent(jsPath)) {
    return compileUtilityModule(bundleDir, bundleName, namespace, outDir, apiVersion);
  }

  const moduleKey = `${namespace}/${bundleName}`;
  const bundleOut = path.join(outDir, moduleKey);
  fs.mkdirSync(bundleOut, { recursive: true });

  {
    const htmlSource = fs.existsSync(htmlPath) ? fs.readFileSync(htmlPath, "utf8") : "<template></template>";
    const htmlResult = transformTemplate(htmlSource, htmlPath, namespace, bundleName, apiVersion);
    writeCompiled(
      path.join(bundleOut, `${bundleName}.html.js`),
      rewriteStylesheetImports(htmlResult.code, bundleName)
    );
  }

  if (fs.existsSync(cssPath)) {
    const cssSource = fs.readFileSync(cssPath, "utf8");
    const cssResult = transformSync(cssSource, cssPath, transformOptions(namespace, bundleName, apiVersion));
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
  const jsResult = transformSync(jsSource, jsPath, transformOptions(namespace, bundleName, apiVersion));
  const jsCode = rewriteStylesheetImports(jsResult.code, bundleName);

  const entryFile = path.join(bundleOut, `${bundleName}.js`);
  writeCompiled(entryFile, jsCode);
  compileSiblingModules(bundleDir, bundleName, bundleOut);
  compileAdditionalTemplateModules(bundleDir, bundleName, namespace, bundleOut, apiVersion);

  return {
    qualified: `${namespace}:${bundleName}`,
    moduleKey,
    file: entryFile,
    tag: `${namespace}-${kebabCase(bundleName)}`,
  };
}

function compileUtilityModule(bundleDir, bundleName, namespace, outDir, apiVersion) {
  const jsPath = path.join(bundleDir, `${bundleName}.js`);
  const moduleKey = `${namespace}/${bundleName}`;
  const bundleOut = path.join(outDir, moduleKey);
  fs.mkdirSync(bundleOut, { recursive: true });

  const entryFile = path.join(bundleOut, `${bundleName}.js`);
  writeCompiled(entryFile, fs.readFileSync(jsPath, "utf8"));
  compileSiblingModules(bundleDir, bundleName, bundleOut);
  compileAdditionalTemplateModules(bundleDir, bundleName, namespace, bundleOut, apiVersion);

  return {
    qualified: `${namespace}:${bundleName}`,
    moduleKey,
    file: entryFile,
    tag: "",
  };
}

function isCustomRenderComponent(jsPath) {
  const source = fs.readFileSync(jsPath, "utf8");
  return /from\s+["']\.\/[^"']+\.html["']/.test(source);
}

function compileSiblingModules(bundleDir, bundleName, bundleOut) {
  walkBundleFiles(bundleDir, (sourcePath) => {
    const rel = path.relative(bundleDir, sourcePath);
    if (!rel.endsWith(".js")) {
      return;
    }
    if (rel === `${bundleName}.js`) {
      return;
    }
    const jsSource = fs.readFileSync(sourcePath, "utf8");
    writeCompiled(path.join(bundleOut, rel), jsSource);
  });
}

function compileAdditionalTemplateModules(bundleDir, bundleName, namespace, bundleOut, apiVersion) {
  walkBundleFiles(bundleDir, (sourcePath) => {
    const rel = path.relative(bundleDir, sourcePath);
    if (rel === `${bundleName}.html`) {
      return;
    }
    if (!rel.endsWith(".html")) {
      return;
    }
    const templateName = path.basename(rel, ".html");
    const htmlSource = fs.readFileSync(sourcePath, "utf8");
    const htmlResult = transformTemplate(htmlSource, sourcePath, namespace, templateName, apiVersion);
    writeCompiled(
      path.join(bundleOut, `${rel}.js`),
      rewriteTemplateRelativeImports(htmlResult.code)
    );

    const cssPath = sourcePath.slice(0, -".html".length) + ".css";
    if (fs.existsSync(cssPath)) {
      compileCSSModule(cssPath, templateName, namespace, bundleDir, bundleOut, apiVersion);
    } else {
      writeCompiled(
        path.join(bundleOut, `${path.relative(bundleDir, cssPath)}.js`),
        "export default '';\n"
      );
    }
    const scopedCSSPath = sourcePath.slice(0, -".html".length) + ".scoped.css";
    writeCompiled(
      path.join(bundleOut, `${path.relative(bundleDir, scopedCSSPath)}.js`),
      "export default '';\n"
    );
  });
}

function compileCSSModule(cssPath, name, namespace, bundleDir, bundleOut, apiVersion) {
  const cssSource = fs.readFileSync(cssPath, "utf8");
  const cssResult = transformSync(cssSource, cssPath, transformOptions(namespace, name, apiVersion));
  const rel = path.relative(bundleDir, cssPath);
  writeCompiled(path.join(bundleOut, `${rel}.js`), cssResult.code);
}

function preflightBundleImports(bundleDir, apiVersion, availability) {
  walkBundleFiles(bundleDir, (sourcePath) => {
    if (!sourcePath.endsWith(".js")) {
      return;
    }
    const source = fs.readFileSync(sourcePath, "utf8");
    const ast = parse(source, {
      sourceType: "module",
      plugins: ["decorators-legacy", "classProperties", "classPrivateProperties", "classPrivateMethods"],
    });
    walkAST(ast, (node) => {
      let specifier = null;
      if ((node.type === "ImportDeclaration" || node.type === "ExportNamedDeclaration" || node.type === "ExportAllDeclaration") && node.source?.type === "StringLiteral") {
        specifier = node.source.value;
      } else if (node.type === "CallExpression" && node.callee?.type === "Import" && node.arguments?.[0]?.type === "StringLiteral") {
        specifier = node.arguments[0].value;
      } else if (node.type === "ImportExpression" && node.source?.type === "StringLiteral") {
        specifier = node.source.value;
      }
      const range = specifier && availability?.[specifier];
      if (!range) {
        return;
      }
      if (range.since && apiVersion < range.since) {
        throw new Error(`LWC module "${specifier}" requires API version ${range.since}.0 or later; bundle uses ${apiVersion}.0`);
      }
      if (range.until && apiVersion >= range.until) {
        throw new Error(`LWC module "${specifier}" is unavailable at API version ${apiVersion}.0`);
      }
    });
  });
}

function walkAST(value, visit) {
  if (!value || typeof value !== "object") {
    return;
  }
  if (typeof value.type === "string") {
    visit(value);
  }
  for (const child of Array.isArray(value) ? value : Object.values(value)) {
    walkAST(child, visit);
  }
}

function rewriteTemplateRelativeImports(code) {
  return code
    .replace(
      /from (["'])(\.\/[^"']+\.scoped\.css)\?scoped=true\1/g,
      'from "$2.js"'
    )
    .replace(/from (["'])(\.\/[^"']+\.(?:html|css))\1/g, 'from "$2.js"');
}

function walkBundleFiles(dir, visit) {
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    const entryPath = path.join(dir, entry.name);
    if (entry.isDirectory()) {
      if (entry.name === "__tests__") {
        continue;
      }
      walkBundleFiles(entryPath, visit);
      continue;
    }
    if (entry.isFile()) {
      visit(entryPath);
    }
  }
}

async function main() {
  const config = await readStdin();
  const projectRoot = path.resolve(config.projectRoot || ".");
  const outDir = path.resolve(config.outDir);
  const namespace = (config.namespace || "c").trim() || "c";
  const modules = {};

  const lwcRoots = new Set();
  for (const rel of config.lwcMetaFiles || []) {
    const metaPath = path.join(projectRoot, rel);
    const base = path.basename(metaPath);
    if (base.endsWith(".js-meta.xml")) {
      lwcRoots.add(path.dirname(metaPath));
    }
  }
  if (lwcRoots.size === 0) {
    for (const rel of config.lwcFiles || []) {
      lwcRoots.add(path.dirname(path.join(projectRoot, rel)));
    }
    for (const rel of config.lwcHtmlFiles || []) {
      lwcRoots.add(path.dirname(path.join(projectRoot, rel)));
    }
  }

  fs.mkdirSync(outDir, { recursive: true });
  for (const bundleDir of lwcRoots) {
    const bundleName = path.basename(bundleDir);
    const bundleKey = path.relative(projectRoot, bundleDir).split(path.sep).join("/");
    const apiVersion = config.lwcApiVersions?.[bundleKey];
    if (!Number.isInteger(apiVersion)) {
      throw new Error(`missing component API version for bundle ${bundleKey}`);
    }
    const entry = compileBundle(bundleDir, bundleName, namespace, outDir, apiVersion, config.lwcModuleAvailability || {});
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
