#!/usr/bin/env node
import fs from "node:fs";
import path from "node:path";
import { spawnSync } from "node:child_process";

const repoRoot = path.resolve(new URL("..", import.meta.url).pathname);
const localInputRoot = path.join(repoRoot, "example-projects", "stubs", "apex-system-stubs");
const siblingInputRoot = path.resolve(repoRoot, "..", "oaer", "example-projects", "stubs", "apex-system-stubs");
const inputRoot = process.argv[2]
  ? path.resolve(process.argv[2])
  : fs.existsSync(localInputRoot)
    ? localInputRoot
    : siblingInputRoot;
const outputFile = process.argv[3]
  ? path.resolve(process.argv[3])
  : path.join(repoRoot, "internal", "typesys", "system_stub_symbols_generated.go");

const systemNamespace = "System";

function goString(value) {
  return JSON.stringify(value ?? "");
}

function goStringSlice(values) {
  if (!values || values.length === 0) return "nil";
  return `[]string{${values.map(goString).join(", ")}}`;
}

function splitTopLevel(value, separator = ",") {
  const parts = [];
  let depth = 0;
  let start = 0;
  for (let i = 0; i < value.length; i++) {
    const ch = value[i];
    if (ch === "<") depth++;
    if (ch === ">") depth = Math.max(0, depth - 1);
    if (ch === separator && depth === 0) {
      parts.push(value.slice(start, i).trim());
      start = i + 1;
    }
  }
  const tail = value.slice(start).trim();
  if (tail) parts.push(tail);
  return parts;
}

function normalizeType(typeName) {
  let out = (typeName || "").trim();
  if (!out) return "Object";
  out = out.replace(/\bAPEX_OBJECT\b/g, "Object");
  out = out.replace(/\bANY\b/g, "Object");
  out = out.replace(/\bVoid\b/g, "void");
  out = out.replace(/\bSystem\.([A-Za-z_][A-Za-z0-9_]*)\b/g, "$1");
  out = out.replace(/\s+/g, "");
  return out;
}

function parameterType(param) {
  param = param.trim();
  if (!param) return "";
  let depth = 0;
  for (let i = 0; i < param.length; i++) {
    const ch = param[i];
    if (ch === "<") depth++;
    if (ch === ">") depth = Math.max(0, depth - 1);
    if (/\s/.test(ch) && depth === 0) {
      return normalizeType(param.slice(0, i));
    }
  }
  return normalizeType(param);
}

function parameterTypes(params) {
  if (!params.trim()) return [];
  return splitTopLevel(params).map(parameterType).filter(Boolean);
}

function declarationName(namespace, className) {
  if (namespace === systemNamespace) return className;
  if (namespace === className) return className;
  return `${namespace}.${className}`;
}

function collectStubFiles(root) {
  const files = [];
  function walk(dir) {
    for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
      const full = path.join(dir, entry.name);
      if (entry.isDirectory()) {
        walk(full);
      } else if (entry.isFile() && entry.name.endsWith(".cls")) {
        files.push(full);
      }
    }
  }
  walk(root);
  files.sort();
  return files;
}

function parseStub(filePath) {
  const source = fs.readFileSync(filePath, "utf8");
  const rel = path.relative(inputRoot, filePath);
  const namespace = rel.split(path.sep)[0];
  const decl = source.match(/\bglobal\s+(?:(?:abstract|virtual|with\s+sharing|without\s+sharing)\s+)*(class|interface|enum)\s+([A-Za-z_][A-Za-z0-9_]*)(?:\s+extends\s+([A-Za-z_][A-Za-z0-9_.<>]*))?(?:\s+implements\s+([A-Za-z_][A-Za-z0-9_.<>,\s]*))?/);
  if (!decl || !namespace) return null;

  const kindWord = decl[1];
  const className = decl[2];
  const name = declarationName(namespace, className);
  const spec = {
    name,
    kind: kindWord === "interface" ? "DeclarationInterface" : kindWord === "enum" ? "DeclarationEnum" : "",
    superClass: normalizeType(decl[3] || ""),
    interfaces: splitTopLevel(decl[4] || "").map(normalizeType).filter(Boolean),
    constructors: [],
    methods: [],
    properties: [],
  };

  const lines = source.split(/\r?\n/);
  const missingTypeProperties = [];
  for (const line of lines) {
    const ctor = line.match(/^\s*(?:global|public)\s+([A-Za-z_][A-Za-z0-9_]*)\s*\(([^)]*)\)/);
    if (ctor && ctor[1] === className) {
      spec.constructors.push(parameterTypes(ctor[2]));
      continue;
    }

    const method = line.match(/^\s*(?:global|public)\s+(static\s+)?([A-Za-z_][A-Za-z0-9_.]*(?:<[^;{}()]+>)?)\s+([A-Za-z_][A-Za-z0-9_]*)\s*\(([^)]*)\)/);
    if (method && method[3] !== className) {
      spec.methods.push({
        name: method[3],
        returnType: normalizeType(method[2]),
        parameters: parameterTypes(method[4]),
        static: Boolean(method[1]),
      });
      continue;
    }

    const prop = line.match(/^\s*(?:global|public)\s+(static\s+)?(?:(.*?)\s+)?([A-Za-z_][A-Za-z0-9_]*)\s*\{\s*get;/);
    if (prop) {
      const rawType = (prop[2] || "").trim();
      const property = {
        name: prop[3],
        type: normalizeType(rawType),
        static: Boolean(prop[1]),
        missingType: rawType === "",
      };
      spec.properties.push(property);
      if (property.missingType) missingTypeProperties.push(property);
    }
  }

  const looksLikeEnum =
    spec.kind === "DeclarationEnum" ||
    (missingTypeProperties.length > 0 &&
      spec.methods.some((m) => m.static && m.name === "valueOf" && m.returnType === name) &&
      spec.methods.some((m) => m.static && m.name === "values" && m.returnType === `List<${name}>`)) ||
    (missingTypeProperties.length > 0 &&
      missingTypeProperties.every((prop) => /^[A-Z][A-Z0-9_]*$/.test(prop.name)) &&
      spec.methods.every((method) => ["clone", "equals", "hashCode", "ordinal", "toString"].includes(method.name)));
  inferMissingPropertyShape(spec, missingTypeProperties, looksLikeEnum);
  if (looksLikeEnum) {
    spec.kind = "DeclarationEnum";
    spec.constructors = [];
    for (const prop of missingTypeProperties) {
      if (/^[A-Z][A-Z0-9_]*$/.test(prop.name)) {
        prop.type = name;
        prop.static = true;
      }
    }
  }
  for (const prop of spec.properties) {
    delete prop.missingType;
  }

  dedupeSpec(spec);
  return spec;
}

function inferMissingPropertyShape(spec, missingTypeProperties, looksLikeEnum) {
  const zeroArgMethods = new Map();
  for (const method of spec.methods) {
    if (method.parameters.length === 0 && method.returnType) {
      zeroArgMethods.set(method.name.toLowerCase(), method);
    }
  }
  for (const prop of missingTypeProperties) {
    const getterKey = `get${capitalizeIdentifier(prop.name)}`.toLowerCase();
    const booleanGetterKey = `is${capitalizeIdentifier(prop.name)}`.toLowerCase();
    const inferredGetter = zeroArgMethods.get(getterKey);
    const inferredBooleanGetter = zeroArgMethods.get(booleanGetterKey);
    if (inferredGetter) {
      prop.type = inferredGetter.returnType;
      prop.static = inferredGetter.static;
      continue;
    }
    if (inferredBooleanGetter) {
      prop.type = inferredBooleanGetter.returnType;
      prop.static = inferredBooleanGetter.static;
      continue;
    }
    if (looksLikeEnum) {
      continue;
    }
    if (/^[A-Z][A-Z0-9_]*$/.test(prop.name)) {
      prop.static = true;
    }
  }
}

function capitalizeIdentifier(value) {
  if (!value) return "";
  return value[0].toUpperCase() + value.slice(1);
}

function constructorKey(params) {
  return params.map((p) => p.toLowerCase()).join(",");
}

function methodKey(method) {
  return `${method.name.toLowerCase()}|${method.static}|${constructorKey(method.parameters)}`;
}

function propertyKey(prop) {
  return `${prop.name.toLowerCase()}|${prop.static}`;
}

function dedupeSpec(spec) {
  spec.constructors = uniqueBy(spec.constructors, constructorKey);
  spec.methods = uniqueBy(spec.methods, methodKey);
  spec.properties = uniqueBy(spec.properties, propertyKey);
  spec.methods.sort((a, b) => methodKey(a).localeCompare(methodKey(b)));
  spec.properties.sort((a, b) => propertyKey(a).localeCompare(propertyKey(b)));
}

function uniqueBy(values, keyFn) {
  const out = [];
  const seen = new Set();
  for (const value of values) {
    const key = keyFn(value);
    if (seen.has(key)) continue;
    seen.add(key);
    out.push(value);
  }
  return out;
}

function writeSpec(spec) {
  let out = "\t{\n";
  out += `\t\tName: ${goString(spec.name)},\n`;
  if (spec.kind) out += `\t\tKind: apexast.${spec.kind},\n`;
  if (spec.superClass) out += `\t\tSuperClass: ${goString(spec.superClass)},\n`;
  if (spec.interfaces.length) out += `\t\tInterfaces: ${goStringSlice(spec.interfaces)},\n`;
  if (spec.constructors.length) {
    out += "\t\tConstructors: [][]string{\n";
    for (const ctor of spec.constructors) {
      out += `\t\t\t${goStringSlice(ctor)},\n`;
    }
    out += "\t\t},\n";
  }
  if (spec.methods.length) {
    out += "\t\tMethods: []StandardMethodSpec{\n";
    for (const method of spec.methods) {
      out += `\t\t\t{Name: ${goString(method.name)}, ReturnType: ${goString(method.returnType)}`;
      if (method.parameters.length) out += `, Parameters: ${goStringSlice(method.parameters)}`;
      if (method.static) out += ", Static: true";
      out += "},\n";
    }
    out += "\t\t},\n";
  }
  if (spec.properties.length) {
    out += "\t\tProperties: []StandardPropertySpec{\n";
    for (const prop of spec.properties) {
      out += `\t\t\t{Name: ${goString(prop.name)}, Type: ${goString(prop.type)}`;
      if (prop.static) out += ", Static: true";
      out += "},\n";
    }
    out += "\t\t},\n";
  }
  out += "\t},\n";
  return out;
}

function referencedTypeNames(typeName) {
  const out = [];
  const tokens = String(typeName || "").match(/[A-Za-z_][A-Za-z0-9_.]*/g) || [];
  for (const token of tokens) {
    if (isBuiltinTypeName(token)) continue;
    out.push(token);
  }
  return out;
}

function isBuiltinTypeName(typeName) {
  const key = String(typeName || "").toLowerCase();
  return new Set([
    "accesslevel",
    "any",
    "blob",
    "boolean",
    "date",
    "datetime",
    "decimal",
    "double",
    "id",
    "integer",
    "list",
    "long",
    "map",
    "object",
    "set",
    "sobject",
    "string",
    "time",
    "void",
  ]).has(key);
}

function addReferencedPlaceholders(specs) {
  const known = new Set(specs.map((spec) => spec.name.toLowerCase()));
  const additions = new Map();
  const consider = (typeName) => {
    for (const name of referencedTypeNames(typeName)) {
      const key = name.toLowerCase();
      if (known.has(key) || additions.has(key)) continue;
      additions.set(key, { name, kind: "", superClass: "", interfaces: [], constructors: [], methods: [], properties: [] });
    }
  };
  for (const spec of specs) {
    consider(spec.superClass);
    for (const iface of spec.interfaces) consider(iface);
    for (const ctor of spec.constructors) {
      for (const param of ctor) consider(param);
    }
    for (const method of spec.methods) {
      consider(method.returnType);
      for (const param of method.parameters) consider(param);
    }
    for (const prop of spec.properties) consider(prop.type);
  }
  return specs.concat([...additions.values()]).sort((a, b) => a.name.localeCompare(b.name));
}

const specs = addReferencedPlaceholders(collectStubFiles(inputRoot)
  .map(parseStub)
  .filter(Boolean)
  .sort((a, b) => a.name.localeCompare(b.name)));

let out = `// Code generated by scripts/generate-system-stub-symbols.mjs; DO NOT EDIT.\n\n`;
out += `package typesys\n\n`;
out += `import "github.com/open-aer/oaer/internal/apexast"\n\n`;
out += `var systemStubSymbolSpecs = []StandardSymbolSpec{\n`;
for (const spec of specs) out += writeSpec(spec);
out += `}\n`;

fs.writeFileSync(outputFile, out);
const gofmt = spawnSync("gofmt", ["-w", outputFile], { stdio: "inherit" });
if (gofmt.status !== 0) process.exit(gofmt.status ?? 1);
