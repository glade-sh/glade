#!/usr/bin/env node
import fs from "node:fs";
import path from "node:path";
import { format, inspect } from "node:util";
import { createRequire } from "node:module";
import { fileURLToPath } from "node:url";
import { salesforceImportSpecifier, validateSalesforceImport } from "./salesforce-imports.mjs";
import { validateGraphQLDocuments } from "./graphql-validation.mjs";
import { validateObjectMetadataAdapters } from "./object-metadata-adapters.mjs";

const defaultToolchainDir = path.dirname(fileURLToPath(import.meta.url));
const toolchainDir = process.env.GLADE_LWC_TOOLCHAIN_DIR || defaultToolchainDir;
const requireFromToolchain = createRequire(path.join(toolchainDir, "compile.mjs"));
const { parse } = requireFromToolchain("@babel/parser");
const { default: traverse } = requireFromToolchain("@babel/traverse");
// The Salesforce import-reference pass only parses. main() loads the compiler
// before any bundle compiles, so that pass skips its startup cost.
let transformSync;
let parseTemplate;
let getAPIVersionFromNumber;
function loadCompiler() {
  ({ transformSync } = requireFromToolchain("@lwc/compiler"));
  ({ parse: parseTemplate } = requireFromToolchain("@lwc/template-compiler"));
  ({ getAPIVersionFromNumber } = requireFromToolchain("@lwc/shared"));
}

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
  return name.replace(/([a-z0-9])([A-Z])/g, "$1-$2").toLowerCase();
}

function writeCompiled(outFile, code) {
  fs.mkdirSync(path.dirname(outFile), { recursive: true });
  fs.writeFileSync(outFile, code, "utf8");
}

function assertCompilerPreservesAPIVersion(apiVersion) {
  const normalized = getAPIVersionFromNumber(apiVersion);
  if (normalized !== apiVersion) {
    throw new Error(
      `LWC compiler cannot preserve declared API version ${apiVersion}.0; it resolves to ${normalized}.0`
    );
  }
}

function transformOptions(namespace, name, apiVersion, capabilities = []) {
  return {
    namespace,
    name,
    apiVersion,
    enableLwcOn: true,
    enableDynamicComponents: capabilities.includes("lightning__dynamicComponent"),
    // Keep parsing enabled so the compiler itself enforces the API-66 gate.
    // Native c_event_call answers at 59/63/65 report LWC1210, not the older
    // flag-disabled LWC1060 diagnostic; 66/67 report LWC1208.
    experimentalComplexExpressions: true,
  };
}

function transformTemplate(source, filename, namespace, name, apiVersion, capabilities = []) {
  let result;
  try {
    result = transformSync(source, filename, transformOptions(namespace, name, apiVersion, capabilities));
  } catch (error) {
    // Native call/optional/computed-expression version and quoting rejections use
    // LWC1535. The c_error_value_computed case captures the computed variants at
    // APIs 59-67, and the c_*_optional cases capture ChainExpression at 59/67.
    // Preserve the complete inner diagnostic and source API.
    const expressionDiagnostic = error.message?.match(/Invalid expression \{[^\r\n]+\} - LWC(?:(?:1208|1210): Template expression doesn't allow (?:CallExpression|ChainExpression)|(?:1207|1209): Template expression doesn't allow computed property access)[^\r\n]*$/);
    if (expressionDiagnostic) {
      throw withDeploymentEnvelope(error, `LWC1535: Unexpected plugin compilation error: Plugin - lwc, Hook - transform, Cause - ${expressionDiagnostic[0]}`);
    }
    throw error;
  }
  validateLegacyDynamicDirective(result, source, filename, namespace, name, apiVersion, capabilities);
  const unsupportedDetailsName = result.warnings?.find(
    (warning) => warning.code === 1057 && warning.message.includes("name is not valid attribute for details")
  );
  if (apiVersion < 67) {
    if (unsupportedDetailsName) {
      throw new Error(unsupportedDetailsName.message);
    }
    // @lwc/compiler 9.x stopped emitting the warning while still accepting
    // the markup. Keep Glade's version gate explicit until the compiler
    // exposes a version-aware diagnostic again.
    if (/<details\b[^>]*\bname\s*=/i.test(source)) {
      throw new Error("name is not valid attribute for details before API version 67.0");
    }
  }
  return result;
}

function transformJavaScript(source, filename, namespace, name, apiVersion) {
  try {
    return transformSync(source, filename, transformOptions(namespace, name, apiVersion));
  } catch (error) {
    throw wireDeploymentDiagnostic(error, source, filename, namespace, name, apiVersion);
  }
}

// Native component code cannot inspect a base button's shadow, while the
// page-level DOM observer can. Guard property reads after compilation so the
// native source and deployment diagnostics retain their original positions.
// This bounded view is not a general-purpose JavaScript sandbox.
function rewriteComponentShadowReads(code, namespace) {
  if (namespace === "lightning" || !/shadowRoot|\\[ux]/.test(code)) return code;
  const ast = parse(code, {
    sourceType: "module",
    // Utility and sibling modules retain their source syntax. Accept the same
    // grammar as preflight; this read pass must not reject an accepted module.
    plugins: [["decorators", { decoratorsBeforeExport: true }], "classProperties", "classPrivateProperties", "classPrivateMethods"],
  });
  const types = requireFromToolchain("@babel/types");
  let helper;
  let changed = false;
  function isWriteTarget(member) {
    let target = member;
    for (let parent = target.parentPath; parent; parent = target.parentPath) {
      if (parent.isArrayPattern() || parent.isObjectPattern() || parent.isRestElement() ||
          (parent.isObjectProperty() && parent.parentPath.isObjectPattern() && parent.node.value === target.node)) {
        target = parent;
        continue;
      }
      return (parent.isAssignmentExpression() || parent.isAssignmentPattern() ||
        parent.isForInStatement() || parent.isForOfStatement()) && parent.node.left === target.node;
    }
    return false;
  }
  function guardRead(member) {
    const { node, parent } = member;
    const property = node.computed ? node.property.value : node.property.name;
    if (property !== "shadowRoot" || types.isSuper(node.object)) return;
    // Do not sever a short circuit inherited from an earlier optional link
    // (a?.b.shadowRoot). Only guard a direct read or its own optional link.
    if (types.isOptionalMemberExpression(node) && !node.optional) return;
    if (!member.isReferenced() || isWriteTarget(member) ||
        types.isUpdateExpression(parent) ||
        (types.isUnaryExpression(parent) && parent.operator === "delete")) return;
    node.object = types.callExpression(types.cloneNode(helper), [node.object]);
    changed = true;
  }
  traverse(ast, {
    Program(program) {
      helper = program.scope.generateUidIdentifier("componentShadowRootReceiver");
    },
    MemberExpression: { exit: guardRead },
    OptionalMemberExpression: { exit: guardRead },
  });
  if (!changed) return code;
  ast.program.body.unshift(types.importDeclaration([
    types.importSpecifier(helper, types.identifier("componentShadowRootReceiver")),
  ], types.stringLiteral("/lightning/runtime/shims/component-dom.mjs")));
  const { default: generate } = requireFromToolchain("@babel/generator");
  return generate(ast, {}, code).code;
}

// The API-59/67 native wire/API and duplicate-wire controls report a formatted
// source frame under this logical service filename. It is diagnostic text only:
// no files are read or written there. Keep this compatibility path restricted to
// the captured CurrentPageReference error classes and APIs; successful
// compilation and other adapters keep their original diagnostics/source bytes.
function wireDeploymentDiagnostic(error, source, filename, namespace, name, apiVersion) {
  if (![59, 67].includes(apiVersion) || ![1095, 1105].includes(error?.code) || !error.location) return error;
  const ast = parse(source, {
    sourceType: "module",
    plugins: [["decorators", { decoratorsBeforeExport: true }], "classProperties", "classPrivateProperties", "classPrivateMethods"],
  });
  const wireNames = new Set();
  const navigationAdapters = new Set();
  for (const statement of ast.program.body) {
    if (statement.type !== "ImportDeclaration") continue;
    for (const binding of statement.specifiers) {
      if (binding.type !== "ImportSpecifier") continue;
      if (statement.source.value === "lwc" && binding.imported.name === "wire") wireNames.add(binding.local.name);
      if (statement.source.value === "lightning/navigation" && binding.imported.name === "CurrentPageReference") navigationAdapters.add(binding.local.name);
    }
  }
  let navigationConflict = false;
  walkAST(ast, (node) => {
    for (const decorator of node.decorators || []) {
      const expression = decorator.expression;
      const start = decorator.loc.start;
      if (start.line !== error.location.line || start.column !== error.location.column) continue;
      if (expression.type === "CallExpression" && expression.callee.type === "Identifier" && wireNames.has(expression.callee.name) &&
          expression.arguments[0]?.type === "Identifier" && navigationAdapters.has(expression.arguments[0].name)) navigationConflict = true;
    }
  });
  if (!navigationConflict) return error;
  const { default: generate } = requireFromToolchain("@babel/generator");
  const formatted = generate(ast, {}, source).code;
  const diagnosticFilename = `/home/sfdc/tools/sfdc-lwc-compiler/14.192.8388608/${path.basename(filename)}`;
  try {
    transformSync(formatted, diagnosticFilename, transformOptions(namespace, name, apiVersion));
  } catch (reported) {
    // Formatting must preserve the actual compiler rejection. Its location and
    // complete message, including the code frame, supply the deployment text.
    if (reported.code === error.code && reported.location) {
      const { line, column } = reported.location;
      const deployed = new Error(`[Line: ${line}, Col: ${column}] LWC1535: Unexpected plugin compilation error: Plugin - lwc, Hook - transform, Cause - ${reported.message}`, { cause: error });
      deployed.code = 1535;
      return deployed;
    }
  }
  return error;
}

function validateLegacyDynamicDirective(result, source, filename, namespace, name, apiVersion, capabilities) {
  if (!result.warnings?.some((warning) => warning.code === 1187)) return;
  // The public parser retains the directive's attribute location; the
  // compiler's deprecation warning instead points at the containing element.
  // Native c_dynamic_legacy rejects the directive itself with LWC1527.
  const parsed = parseTemplate(source, {
    ...transformOptions(namespace, name, apiVersion, capabilities),
    experimentalDynamicDirective: true,
  });
  walkAST(parsed.root, (node) => {
    if (node.type !== "Directive" || node.name !== "Dynamic") return;
    const error = new Error("LWC1527: The lwc:dynamic attribute is no longer supported. Please use the new dynamic component syntax instead. See https://lwc.dev/guide/html_templates#import-and-instantiate-a-component-dynamically for more details.");
    error.code = 1527;
    error.filename = filename;
    error.location = { line: node.location.startLine, column: node.location.startColumn };
    throw error;
  });
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

function compileBundle(bundleDir, bundleName, namespace, outDir, apiVersion, moduleAvailability, configuredProperties, salesforceImports, capabilities, lightningModules) {
  const jsPath = path.join(bundleDir, `${bundleName}.js`);
  const htmlPath = path.join(bundleDir, `${bundleName}.html`);
  const cssPath = path.join(bundleDir, `${bundleName}.css`);
  if (!fs.existsSync(jsPath)) {
    return null;
  }
  assertCompilerPreservesAPIVersion(apiVersion);
  preflightBundleImports(bundleDir, bundleName, apiVersion, moduleAvailability, configuredProperties, salesforceImports, lightningModules, namespace);
  // GraphQL document validation is independent of the shared wire preflight.
  walkBundleFiles(bundleDir, sourcePath => {
    if (!sourcePath.endsWith(".js")) return;
    const ast = parse(fs.readFileSync(sourcePath, "utf8"), {
      sourceType: "module",
      plugins: [["decorators", { decoratorsBeforeExport: true }], "classProperties", "classPrivateProperties", "classPrivateMethods"],
    });
    validateGraphQLDocuments(ast, salesforceImports?.schema, walkAST);
  });
  if (!fs.existsSync(htmlPath) && !isCustomRenderComponent(jsPath)) {
    return compileUtilityModule(bundleDir, bundleName, namespace, outDir, apiVersion, capabilities);
  }

  const moduleKey = `${namespace}/${bundleName}`;
  const bundleOut = path.join(outDir, moduleKey);
  fs.mkdirSync(bundleOut, { recursive: true });

  {
    const htmlSource = fs.existsSync(htmlPath) ? fs.readFileSync(htmlPath, "utf8") : "<template></template>";
    const htmlResult = transformTemplate(htmlSource, htmlPath, namespace, bundleName, apiVersion, capabilities);
    writeCompiled(
      path.join(bundleOut, `${bundleName}.html.js`),
      rewriteStylesheetImports(htmlResult.code, bundleName)
    );
  }

  if (fs.existsSync(cssPath)) {
    const cssResult = transformStylesheet(cssPath, namespace, bundleName, apiVersion);
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
  const jsResult = transformJavaScript(jsSource, jsPath, namespace, bundleName, apiVersion);
  const jsCode = rewriteComponentShadowReads(rewriteStylesheetImports(jsResult.code, bundleName), namespace);

  const entryFile = path.join(bundleOut, `${bundleName}.js`);
  writeCompiled(entryFile, jsCode);
  compileSiblingModules(bundleDir, bundleName, bundleOut, namespace);
  compileAdditionalTemplateModules(bundleDir, bundleName, namespace, bundleOut, apiVersion, capabilities);

  return {
    qualified: `${namespace}:${bundleName}`,
    moduleKey,
    file: entryFile,
    tag: `${namespace}-${kebabCase(bundleName)}`,
  };
}

function compileUtilityModule(bundleDir, bundleName, namespace, outDir, apiVersion, capabilities) {
  const jsPath = path.join(bundleDir, `${bundleName}.js`);
  const moduleKey = `${namespace}/${bundleName}`;
  const bundleOut = path.join(outDir, moduleKey);
  fs.mkdirSync(bundleOut, { recursive: true });

  const entryFile = path.join(bundleOut, `${bundleName}.js`);
  writeCompiled(entryFile, rewriteComponentShadowReads(fs.readFileSync(jsPath, "utf8"), namespace));
  compileSiblingModules(bundleDir, bundleName, bundleOut, namespace);
  compileAdditionalTemplateModules(bundleDir, bundleName, namespace, bundleOut, apiVersion, capabilities);

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

function compileSiblingModules(bundleDir, bundleName, bundleOut, namespace) {
  walkBundleFiles(bundleDir, (sourcePath) => {
    const rel = path.relative(bundleDir, sourcePath);
    if (!rel.endsWith(".js")) {
      return;
    }
    if (rel === `${bundleName}.js`) {
      return;
    }
    const jsSource = fs.readFileSync(sourcePath, "utf8");
    writeCompiled(path.join(bundleOut, rel), rewriteComponentShadowReads(jsSource, namespace));
  });
}

function compileAdditionalTemplateModules(bundleDir, bundleName, namespace, bundleOut, apiVersion, capabilities) {
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
    const htmlResult = transformTemplate(htmlSource, sourcePath, namespace, templateName, apiVersion, capabilities);
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
  const cssResult = transformStylesheet(cssPath, namespace, name, apiVersion);
  const rel = path.relative(bundleDir, cssPath);
  writeCompiled(path.join(bundleOut, `${rel}.js`), cssResult.code);
}

function transformStylesheet(cssPath, namespace, name, apiVersion) {
  const source = fs.readFileSync(cssPath, "utf8");
  if (source.length === 0) {
    throw new Error(`Lightning Component Resource [lwc/${name}/${path.basename(cssPath)}] for Lightning Component Bundle ${name} cannot be empty.`);
  }
  let result;
  try {
    result = transformSync(source, cssPath, transformOptions(namespace, name, apiVersion));
  } catch (error) {
    throw stylesheetDeploymentError(error, source, cssPath, namespace, name, apiVersion);
  }
  // CSS compilation emits module imports without linking them. Apply native
  // stylesheet linking checks to those imports, preserving the parsed text.
  const ast = parse(result.code, { sourceType: "module" });
  for (const node of ast.program.body) {
    if (node.type !== "ImportDeclaration") continue;
    const specifier = node.source.value;
    if (specifier.startsWith("./") || specifier.startsWith("../")) {
      const target = path.resolve(path.dirname(cssPath), specifier);
      if (!fs.existsSync(target) || !fs.statSync(target).isFile()) {
        throw new Error(`LWC1011: Failed to resolve import "${specifier}" from "${path.basename(cssPath)}". Please add "${path.basename(specifier)}" file to the component folder.`);
      }
    } else if (specifier.includes(".") || specifier.includes(":")) {
      throw new Error(`Invalid import '${specifier}'. No '.' in module reference or start relative path with ./`);
    }
  }
  return result;
}

function stylesheetDeploymentError(error, source, filename, namespace, name, apiVersion) {
  if (error.code !== 1009 || !error.message?.startsWith("CssSyntaxError: LWC1009: ")) return error;
  const syntax = /: Unclosed (?:block|comment)$/.test(error.message);
  const selector = /: Invalid usage of (?:id selector |unsupported selector ":(?:host-context|root)"\.)/.test(error.message);
  if (!syntax && !selector) return error;
  // Native CSS diagnostics use a logical compiler-service filename. Re-run
  // only the rejected transform to obtain its actual diagnostic under that
  // identity; successful CSS output and original diagnostic objects stay intact.
  const deployedFilename = `/home/sfdc/tools/sfdc-lwc-compiler/14.192.8388608/${path.basename(filename)}`;
  try {
    transformSync(source, deployedFilename, transformOptions(namespace, name, apiVersion));
  } catch (reported) {
    if (reported.code !== error.code || !reported.message?.startsWith("CssSyntaxError: LWC1009: ")) return error;
    const message = syntax
      ? `LWC1709: Syntax error encountered while parsing file ${path.basename(filename)}. Cause: ${reported.message.slice("CssSyntaxError: LWC1009: ".length)}`
      : `LWC1535: Unexpected plugin compilation error: Plugin - lwc, Hook - transform, Cause - ${reported.message}`;
    return withPositionedDeploymentEnvelope(error, message);
  }
  return error;
}

function preflightBundleImports(bundleDir, bundleName, apiVersion, availability, configuredProperties, salesforceImports, lightningModules, namespace) {
  walkBundleFiles(bundleDir, (sourcePath) => {
    if (!sourcePath.endsWith(".js")) {
      return;
    }
    const source = fs.readFileSync(sourcePath, "utf8");
    const parserOptions = {
      sourceType: "module",
      // Match the parser grammar used by @lwc/compiler. Legacy decorators
      // incorrectly consume the computed member key in @api ['method']().
      plugins: [["decorators", { decoratorsBeforeExport: true }], "classProperties", "classPrivateProperties", "classPrivateMethods"],
    };
    let ast;
    try {
      ast = parse(source, parserOptions);
    } catch (error) {
      // Native syntax/import binding failures retain Babel's text and position
      // inside the Metadata API envelope.
      if (["UnexpectedToken", "VarRedeclaration"].includes(error.reasonCode) && error.loc) {
        throw deploymentDiagnostic(`Parsing error: ${error.message}`, error.loc);
      }
      if (error.reasonCode === "DecoratorConstructor") {
        // Recover only to identify a wire constructor. Other decorator/parser
        // failures retain their original diagnostics and rejection order.
        let recovered;
        try {
          recovered = parse(source, { ...parserOptions, errorRecovery: true });
        } catch (_) {
          throw error;
        }
        validateWireConsumerDeclarations(recovered, error);
      }
      throw error;
    }
    validateTrackDecorators(ast);
    validatePublicAPIDecorators(ast);
    validateWireDecorators(ast);
    validateWireConsumerDeclarations(ast);
    validateRecordWireAdapterReferences(ast);
    validateObjectMetadataAdapters(ast, traverse);
    if (sourcePath === path.join(bundleDir, `${bundleName}.js`)) {
      validateConfiguredProperties(ast, configuredProperties);
    }
    walkAST(ast, (node) => {
      const specifier = salesforceImportSpecifier(node);
      if (!specifier) {
        return;
      }
      validateSalesforceImport(node, specifier, sourcePath, salesforceImports);
      if (lightningModules && specifier.startsWith("lightning/") &&
          !Object.prototype.hasOwnProperty.call(lightningModules, specifier)) {
        throw new Error(`No MODULE named markup://lightning:${specifier.slice("lightning/".length)} found : [markup://${namespace}:${bundleName}]`);
      }
      if (specifier === "lwc" && node.type === "ImportDeclaration" && node.specifiers.some(
        (binding) => binding.type === "ImportDefaultSpecifier" || binding.imported?.name === "default"
      )) {
        throw new Error('LWC1503: Invalid import. "lwc" does not have a default export. Use named imports instead.');
      }
      if (specifier === "lwc" && node.type === "ImportDeclaration") {
        const binding = node.specifiers.find((binding) => binding.imported?.name === "isServer");
        if (binding) {
          throw deploymentDiagnostic('Invalid import. "isServer" can\'t be imported from "lwc".', {
            ...binding.loc.start, column: binding.loc.start.column + 1,
          });
        }
      }
      if (specifier.startsWith("./") || specifier.startsWith("../")) {
        validateRelativeImport(specifier, sourcePath, bundleDir);
      }
      // Scoped resource modules require a value after the provider name.
      // The bare apex module also exports named helper functions.
      if (specifier.startsWith("@salesforce/") && specifier !== "@salesforce/apex") {
        const [, provider, ...resource] = specifier.split("/");
        if (!provider || !resource.join("/")) {
          throw new Error(`LWC1704: Missing resource value for ${specifier}`);
        }
      }
      const range = availability?.[specifier];
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

function deploymentDiagnostic(message, location) {
  const error = new Error(`[Line: ${location.line}, Col: ${location.column}] LWC1503: ${message}`);
  error.code = 1503;
  return error;
}

// Captured reference and alias cases require these imported bindings to
// be decorator-only. Resolve lexical references so property keys and shadowed
// locals do not acquire the import's restriction.
function validateRecordWireAdapterReferences(ast) {
  const wireAdapters = new Map([
    ["lightning/uiRecordApi", ["getRecord", "getRecords"]],
    ["lightning/uiRelatedListApi", ["getRelatedListRecords", "getRelatedListCount",
      "getRelatedListInfo", "getRelatedListsInfo", "getRelatedListRecordsBatch", "getRelatedListInfoBatch"]],
    ["lightning/uiListsApi", ["getListInfoByName", "getListInfosByName"]],
    ["lightning/uiListApi", ["getListUi"]],
    ["lightning/uiAppsApi", ["getNavItems"]],
  ]);
  const adapters = [];
  for (const statement of ast.program.body) {
    if (statement.type !== "ImportDeclaration") continue;
    const names = wireAdapters.get(statement.source.value);
    if (!names) continue;
    for (const binding of statement.specifiers) {
      if (binding.type === "ImportSpecifier" && names.includes(binding.imported.name)) {
        adapters.push(binding.local.name);
      }
    }
  }
  if (!adapters.length) return;
  const { traverse } = requireFromToolchain("@babel/core");
  traverse(ast, {
    Program(program) {
      for (const name of adapters) {
        const binding = program.scope.getBinding(name);
        for (const reference of binding.referencePaths) {
          const call = reference.parentPath;
          if (call.isCallExpression() && reference.listKey === "arguments" && reference.key === 0 &&
              call.parentPath.isDecorator() && call.node.callee.type === "Identifier") {
            const decorator = call.scope.getBinding(call.node.callee.name)?.path;
            if (decorator?.isImportSpecifier() && decorator.node.imported.name === "wire" &&
                decorator.parent.source.value === "lwc") continue;
          }
          const { line, column } = reference.node.loc.start;
          throw deploymentDiagnostic(`"${name}" is a wire adapter and can only be used via the @wire decorator.`,
            { line, column: column + 1 });
        }
      }
      program.stop();
    },
  });
}

// Deployment validation missing from the open-source @wire transform. Resolve
// the actual lwc import binding so unrelated methods/decorators stay unchanged.
function validateWireDecorators(ast) {
  const wireNames = new Set();
  const navigationAdapters = new Set();
  for (const statement of ast.program.body) {
    if (statement.type !== "ImportDeclaration") continue;
    for (const binding of statement.specifiers) {
      if (binding.type !== "ImportSpecifier") continue;
      if (statement.source.value === "lwc" && binding.imported.name === "wire") wireNames.add(binding.local.name);
      if (statement.source.value === "lightning/navigation" && binding.imported.name === "CurrentPageReference") navigationAdapters.add(binding.local.name);
    }
  }
  if (!wireNames.size) return;
  walkAST(ast, (node) => {
    for (const decorator of node.decorators || []) {
      const expression = decorator.expression;
      if (expression.type !== "CallExpression" || expression.callee.type !== "Identifier" || !wireNames.has(expression.callee.name)) continue;
      const adapter = expression.arguments[0];
      const start = adapter?.loc.start || decorator.loc.start;
      const location = { line: start.line, column: start.column + 1 };
      const missingAdapter = '"@wire" decorators expect the identifier of an adapter to be passed as first argument.';
      if (!adapter || ["NullLiteral", "StringLiteral", "CallExpression"].includes(adapter.type)) {
        const error = deploymentDiagnostic(missingAdapter, location);
        if (adapter?.type === "CallExpression" && adapter.callee.type === "Identifier" && navigationAdapters.has(adapter.callee.name)) {
          const called = deploymentDiagnostic(`"${adapter.callee.name}" is a wire adapter and can only be used via the @wire decorator.`, location);
          const combined = new Error(`${called.message}\n${error.message}`);
          combined.errors = [called, error];
          throw combined;
        }
        throw error;
      }
      if (node.static) {
        const start = decorator.loc.start;
        throw deploymentDiagnostic('"@wire" decorators can\'t be applied to static properties.', { line: start.line, column: start.column + 1 });
      }
    }
  });
}

function validateRelativeImport(specifier, sourcePath, bundleDir) {
  const target = path.resolve(path.dirname(sourcePath), specifier.split("?")[0]);
  const relative = path.relative(bundleDir, target);
  const inBundle = relative !== ".." && !relative.startsWith(`..${path.sep}`) && !path.isAbsolute(relative);
  const candidates = [target, `${target}.js`, `${target}.html`, `${target}.css`];
  if (!inBundle || !candidates.some((file) => fs.existsSync(file) && fs.statSync(file).isFile())) {
    throw new Error(`LWC1011: Failed to resolve import "${specifier}" from "${path.basename(sourcePath)}". Please add the file to the component folder.`);
  }
}

function validateTrackDecorators(ast) {
  const trackNames = new Set();
  for (const statement of ast.program.body) {
    if (statement.type !== "ImportDeclaration" || statement.source.value !== "lwc") {
      continue;
    }
    for (const binding of statement.specifiers) {
      if (binding.type === "ImportSpecifier" && binding.imported.name === "track") {
        trackNames.add(binding.local.name);
      }
    }
  }
  if (trackNames.size === 0) {
    return;
  }
  walkAST(ast, (node) => {
    for (const decorator of node.decorators || []) {
      const expression = decorator.expression;
      if (expression.type === "CallExpression" && expression.callee.type === "Identifier" && trackNames.has(expression.callee.name)) {
        throw new Error('LWC1503: "@track" decorators don\'t support argument');
      }
      if (expression.type === "Identifier" && trackNames.has(expression.name) &&
          (node.type !== "ClassProperty" || node.static)) {
        throw new Error('LWC1503: "@track" decorators can only be applied to class fields');
      }
    }
  });
}

// Native wire declaration preflight and rejection text (API 59/67).
function validateWireConsumerDeclarations(ast, constructorError) {
  const wireNames = new Set();
  const importedNames = new Set();
  for (const statement of ast.program.body) {
    if (statement.type !== "ImportDeclaration") continue;
    for (const binding of statement.specifiers) {
      importedNames.add(binding.local.name);
      if (statement.source.value === "lwc" && binding.type === "ImportSpecifier" && binding.imported.name === "wire") {
        wireNames.add(binding.local.name);
      }
    }
  }
  if (!wireNames.size) return;
  const reject = (expression, message) => {
    const { line, column } = expression.loc.start;
    throw new Error(`[Line: ${line}, Col: ${column + 1}] ${message}`);
  };
  walkAST(ast, (node) => {
    for (const decorator of node.decorators || []) {
      const expression = decorator.expression;
      if (expression.type !== "CallExpression" || expression.callee.type !== "Identifier" ||
          !wireNames.has(expression.callee.name)) continue;
      if (constructorError) {
        const { line, column } = constructorError.loc;
        if (node.type === "ClassMethod" && node.kind === "constructor" &&
            decorator.loc.start.line === line && decorator.loc.start.column === column) {
          throw new Error(`[Line: ${line}, Col: ${column}] LWC1503: Parsing error: ${constructorError.message}`);
        }
        continue;
      }
      if (node.static) {
        reject(decorator, `LWC1503: "@wire" decorators can't be applied to static properties.`);
      }
      if (node.type !== "ClassProperty" && !(node.type === "ClassMethod" && node.kind === "method")) {
        reject(decorator, 'LWC1503: "@wire" decorators can only be applied to class field and methods.');
      }
      const adapter = expression.arguments[0];
      if (!adapter || adapter.type !== "Identifier") {
        reject(adapter || decorator, 'LWC1503: "@wire" decorators expect the identifier of an adapter to be passed as first argument.');
      }
      // Native undefined-adapter lint failures are a hosted boundary; preserve
      // the compiler's existing local rejection for that unresolved identifier.
      if (adapter.name !== "undefined" && !importedNames.has(adapter.name)) {
        reject(adapter, `LWC1503: "${adapter.name}" is not a known adapter.`);
      }
      const config = expression.arguments[1];
      if (config && config.type !== "ObjectExpression") {
        reject(config, 'LWC1503: "@wire" decorators expect a configuration as an object expression as second argument.');
      }
    }
  });
}

// Public API checks missing from the open-source compiler's deployment path.
// Only bindings imported as api from lwc are decorators of the public surface.
function validatePublicAPIDecorators(ast) {
  const apiNames = new Set();
  for (const statement of ast.program.body) {
    if (statement.type !== "ImportDeclaration" || statement.source.value !== "lwc") continue;
    for (const binding of statement.specifiers) {
      if (binding.type === "ImportSpecifier" && binding.imported.name === "api") {
        apiNames.add(binding.local.name);
      }
    }
  }
  if (!apiNames.size) return;
  walkAST(ast, (node) => {
    if (node.type !== "ClassBody") return;
    const publicNames = new Set();
    for (const member of node.body) {
      let isPublic = false;
      for (const decorator of member.decorators || []) {
        const expression = decorator.expression;
        if (expression.type === "CallExpression" && expression.callee.type === "Identifier" && apiNames.has(expression.callee.name)) {
          throw new Error('LWC1503: "@api" decorators don\'t support argument.');
        }
        if (expression.type === "Identifier" && apiNames.has(expression.name)) isPublic = true;
      }
      if (!isPublic) continue;
      if (member.static) {
        throw new Error('LWC1503: "@api" decorators can only be applied to class fields and methods.');
      }
      const name = !member.computed && member.key.type === "Identifier" ? member.key.name : member.key.type === "StringLiteral" ? member.key.value : null;
      if (name === null) continue;
      if (publicNames.has(name)) {
        throw new Error(`LWC1503: "${name}" has already been declared as a public property.`);
      }
      publicNames.add(name);
      if (member.type !== "ClassProperty") continue;
      if (name.startsWith("on")) {
        throw new Error(`LWC1503: Invalid public property "${name}". Properties starting with "on" are reserved for event handlers.`);
      }
      if (name.startsWith("data") && name.length > 4) {
        throw new Error(`LWC1503: Invalid public property name "${name}". Properties starting with "data" correspond to data-* HTML attributes, which are not allowed.`);
      }
      if (name === "slot" || name === "part") {
        throw new Error(`LWC1503: Invalid public property "${name}". This property name is a reserved property.`);
      }
      if (member.value?.type === "BooleanLiteral" && member.value.value === true) {
        throw new Error(`LWC1503: Invalid public property initialization for "${name}". Boolean public properties should not be initialized to "true", consider initializing the property to "false".`);
      }
    }
  });
}

function validateConfiguredProperties(ast, configuredProperties) {
  if (!configuredProperties?.length) {
    return;
  }
  const statements = ast.program.body;
  const apiNames = new Set();
  for (const statement of statements) {
    if (statement.type === "ImportDeclaration" && statement.source.value === "lwc") {
      for (const binding of statement.specifiers) {
        if (binding.type === "ImportSpecifier" && binding.imported.name === "api") {
          apiNames.add(binding.local.name);
        }
      }
    }
  }
  let component = statements.find((statement) => statement.type === "ExportDefaultDeclaration")?.declaration;
  if (component?.type === "Identifier") {
    const name = component.name;
    component = statements.map((statement) => statement.declaration || statement).find(
      (declaration) => declaration.type === "ClassDeclaration" && declaration.id?.name === name
    );
  }
  const publicProperties = new Set();
  if (component?.type === "ClassDeclaration" || component?.type === "ClassExpression") {
    for (const member of component.body.body) {
      const isProperty = member.type === "ClassProperty" ||
        (member.type === "ClassMethod" && (member.kind === "get" || member.kind === "set"));
      const isPublic = member.decorators?.some(
        (decorator) => decorator.expression.type === "Identifier" && apiNames.has(decorator.expression.name)
      );
      if (isProperty && isPublic && !member.static) {
        if (!member.computed && member.key.type === "Identifier") {
          publicProperties.add(member.key.name);
        } else if (member.key.type === "StringLiteral") {
          publicProperties.add(member.key.value);
        }
      }
    }
  }
  for (const property of configuredProperties) {
    if (!publicProperties.has(property)) {
      throw new Error(`The '${property}' property doesn't exist on the component.`);
    }
  }
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
  const bundles = {};

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

  const schemaReferencesOnly = process.argv.includes("--salesforce-schema-references");
  if (schemaReferencesOnly || process.argv.includes("--salesforce-import-references")) {
    const references = new Set();
    for (const bundleDir of lwcRoots) {
      walkBundleFiles(bundleDir, (sourcePath) => {
        if (!sourcePath.endsWith(".js")) return;
        const source = fs.readFileSync(sourcePath, "utf8");
        let ast;
        try {
          ast = parse(source, {
            sourceType: "module",
            plugins: ["decorators-legacy", "classProperties", "classPrivateProperties", "classPrivateMethods"],
          });
        } catch (_) {
          // The normal preflight reports syntax errors in its original order.
          // This pass only collects references from valid import declarations.
          return;
        }
        walkAST(ast, (node) => {
          const specifier = salesforceImportSpecifier(node);
          if (specifier?.startsWith(schemaReferencesOnly ? "@salesforce/schema/" : "@salesforce/")) references.add(specifier);
        });
      });
    }
    process.stdout.write(JSON.stringify([...references].sort()));
    return;
  }

  // Outside the per-bundle try: a missing toolchain stays a process failure.
  loadCompiler();
  fs.mkdirSync(outDir, { recursive: true });
  for (const bundleDir of lwcRoots) {
    const bundleName = path.basename(bundleDir);
    const bundleKey = path.relative(projectRoot, bundleDir).split(path.sep).join("/");
    const bundleModules = {};
    const bundleOut = config.batch ? config.bundleOutDirs[bundleKey] : outDir;
    try {
      const apiVersion = config.lwcApiVersions?.[bundleKey];
      if (!Number.isInteger(apiVersion)) {
        throw new Error(`missing component API version for bundle ${bundleKey}`);
      }
      const entry = compileBundle(bundleDir, bundleName, namespace, bundleOut, apiVersion, config.lwcModuleAvailability || {}, config.lwcConfiguredProperties?.[bundleKey] || [], config.salesforceImports, config.lwcCapabilities?.[bundleKey] || [], config.lightningModules);
      // Native c_metadata_* checks capability declarations after template
      // compilation; invalid dynamic usage keeps its actual compiler error.
      const capabilityError = config.lwcCapabilityErrors?.[bundleKey];
      if (capabilityError) throw new Error(capabilityError);
      if (entry) {
        bundleModules[entry.qualified] = {
          moduleKey: entry.moduleKey,
          tag: entry.tag,
          file: entry.file,
        };
        if (!config.batch) Object.assign(modules, bundleModules);
      }
      if (config.batch) {
        fs.mkdirSync(bundleOut, { recursive: true });
        fs.writeFileSync(path.join(bundleOut, "manifest.json"), JSON.stringify({ modules: bundleModules }, null, 2), "utf8");
        bundles[bundleKey] = { modules: bundleModules, diagnostics: [], reportedDiagnostics: [] };
      }
    } catch (err) {
      if (!config.batch) throw err;
      // Render the same Error object passed to console.error by Compile.
      // Its original diagnostics stay separate from the deployment envelope.
      const failure = compilationFailure(err);
      const errors = Array.isArray(failure.errors) ? failure.errors : [failure];
      bundles[bundleKey] = {
        modules: {},
        error: format(failure),
        diagnostics: errors.map((diagnostic) => ({
          code: typeof diagnostic.code === "number" ? diagnostic.code : 0,
          message: diagnostic.message,
        })),
        deploymentDiagnostics: deploymentDiagnostics(failure),
        reportedDiagnostics: reportedDiagnostics(failure),
      };
    }
  }

  if (config.batch) {
    process.stdout.write(JSON.stringify({ bundles }));
    return;
  }

  const manifestPath = path.join(outDir, "manifest.json");
  fs.writeFileSync(manifestPath, JSON.stringify({ modules }, null, 2), "utf8");
  process.stdout.write(JSON.stringify({ modules, manifestPath }));
}

// Deployment reporting retains the compiler's location and its public error
// envelope. The underlying code/message objects remain available separately.
function reportedDiagnostics(error) {
  const diagnostics = Array.isArray(error.errors) ? error.errors : [error];
  return diagnostics.map((diagnostic) => {
    const message = diagnostics.length === 1 && error.cause ? error.message : diagnostic.message;
    const location = diagnostic.location;
    if (!(location?.line > 0 && location?.column > 0)) return message;
    const position = `[Line: ${location.line}, Col: ${location.column}] `;
    // withDeploymentEnvelope already includes the original location.
    return message.startsWith(position) ? message : position + message;
  });
}

// Keep the original compiler diagnostics when adding a deployment envelope.
// Both batch consumers and single-compile observers read these same objects.
function withDeploymentEnvelope(error, message) {
  // Syntax and expression errors retain the original parser/template source location.
  const location = error.location ?? error.loc;
  const position = Number.isInteger(location?.line) && Number.isInteger(location?.column)
    ? `[Line: ${location.line}, Col: ${location.column}] `
    : "";
  const wrapped = new Error(position + message, { cause: error });
  wrapped.errors = Array.isArray(error.errors) ? error.errors : [error];
  wrapped.location = error.location || error.loc;
  return wrapped;
}

// Publish the deployment diagnostic from the compiler's actual message and
// source position. Original compiler diagnostics remain available separately.
function deploymentDiagnostics(error) {
  const diagnostics = Array.isArray(error.errors) && error.errors.length > 1 ? error.errors : [error];
  return diagnostics.map((diagnostic) => {
    const message = diagnostic.message;
    const location = diagnostic.location || diagnostic.cause?.location || diagnostic.errors?.[0]?.location;
    if (!message.startsWith("[Line: ") && location?.line > 0 && location?.column > 0) {
      return `[Line: ${location.line}, Col: ${location.column}] ${message}`;
    }
    return message;
  });
}

// Add the captured source position only for the newly covered error classes.
// Existing envelope callers retain their current rendering and diagnostics.
function withPositionedDeploymentEnvelope(error, message) {
  const wrapped = withDeploymentEnvelope(error, message);
  const location = error.location ?? error.loc ?? error.errors?.[0]?.location;
  const position = Number.isInteger(location?.line) && Number.isInteger(location?.column)
    ? `[Line: ${location.line}, Col: ${location.column}] `
    : "";
  wrapped.message = position + message;
  wrapped.location = location;
  return wrapped;
}

// Native parser and public-API diagnostics have a deployment envelope around
// the underlying error. Preserve that complete underlying text.
// The captured reserved-name, duplicate/private/track API, accessor-arity and
// constructor rows back these specific diagnostic classes at API 59 and 67.
// The c_error_method_syntax case backs UnexpectedToken at the same APIs.
function publicAPIDeploymentError(error) {
  const message = error?.message;
  if (typeof message !== "string") return error;
  // Native invalid import aliases and duplicate bindings capture these parser
  // classes at both API floors. Deploy reports the parser's zero-based column
  // unchanged, unlike the one-based wire-adapter reference diagnostics.
  if (error.code === "BABEL_PARSER_SYNTAX_ERROR" &&
      ["UnexpectedToken", "VarRedeclaration"].includes(error.reasonCode) && error.loc) {
    const deploymentMessage = `LWC1503: Parsing error: ${message}`;
    const wrapped = withDeploymentEnvelope(error, deploymentMessage);
    wrapped.errors = [{
      code: 1503,
      message: `[Line: ${error.loc.line}, Col: ${error.loc.column}] ${deploymentMessage}`,
    }];
    return wrapped;
  }
  if (message.startsWith("SyntaxError: ") && /: LWC(?:1093|1096|1103|1110):/.test(message.split("\n")[0])) {
    return withDeploymentEnvelope(error, `LWC1535: Unexpected plugin compilation error: Plugin - lwc, Hook - transform, Cause - ${message}`);
  }
  if (["BadGetterArity", "BadSetterArity", "DecoratorConstructor", "UnexpectedToken"].includes(error.reasonCode)) {
    return withDeploymentEnvelope(error, `LWC1503: Parsing error: ${message}`);
  }
  return error;
}

function compilationFailure(error) {
  const err = publicAPIDeploymentError(error);
  // CompilerError may discard its stack and otherwise prints as an object.
  // Customize only inspection: observers must still receive an Error with
  // its original code/message objects, not an already formatted string.
  if (/^(?:\[Line: [0-9]+, Col: [0-9]+\] )?LWC/.test(err?.message ?? "")) {
    const location = err.location ?? err.errors?.[0]?.location;
    const positioned = /^LWC(?:1704|1705|1051|1122):/.test(err.message)
      && Number.isInteger(location?.line) && Number.isInteger(location?.column);
    const position = positioned ? `[Line: ${location.line}, Col: ${location.column}] ` : "";
    err[inspect.custom] = () => `Error: ${position}${err.message}`;
  }
  return err;
}

main().catch((error) => {
  console.error(compilationFailure(error));
  process.exit(1);
});
