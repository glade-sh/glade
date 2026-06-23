#!/usr/bin/env node
import fs from "node:fs";
import path from "node:path";
import { spawnSync } from "node:child_process";

const repoRoot = path.resolve(new URL("..", import.meta.url).pathname);
const localInputRoot = path.join(repoRoot, "example-projects", "stubs", "apex-system-stubs");
const siblingInputRoot = path.resolve(repoRoot, "..", "glade", "example-projects", "stubs", "apex-system-stubs");
const inputRoot = process.argv[2]
  ? path.resolve(process.argv[2])
  : fs.existsSync(localInputRoot)
    ? localInputRoot
    : siblingInputRoot;
const outputFile = process.argv[3]
  ? path.resolve(process.argv[3])
  : path.join(repoRoot, "internal", "typesys", "system_stub_symbols_generated.go");
const docsContractFile = process.argv[4] ? path.resolve(process.argv[4]) : "";

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

function normalizeContractType(typeName) {
  const raw = (typeName || "").trim();
  if (!raw) return "";
  return normalizeType(raw);
}

function applySignatureOverrides(spec) {
  if (spec.name !== "Database") return;
  for (const method of spec.methods) {
    if (method.name === "countQueryWithBinds") {
      method.returnType = "Integer";
    }
  }
}

function loadDocsContracts(filePath) {
  if (!filePath) return new Map();
  const data = JSON.parse(fs.readFileSync(filePath, "utf8"));
  const contracts = new Map();
  const ensure = (name) => {
    const key = String(name || "").toLowerCase();
    if (!key) return null;
    if (!contracts.has(key)) contracts.set(key, { name, properties: [], methods: [], constructors: [] });
    return contracts.get(key);
  };

  for (const type of data.types || []) {
    const contract = ensure(type.name);
    if (!contract) continue;
    for (const prop of type.properties || []) {
      contract.properties.push({ name: prop.name, type: normalizeContractType(prop.type || prop.propertyType || prop.returnType), static: Boolean(prop.static) });
    }
    for (const method of type.methods || []) {
      contract.methods.push({
        name: method.name,
        returnType: normalizeContractType(method.returnType || method.type),
        parameters: contractParameterSpecs(method.parameters || method.parameterTypes || []),
        static: Boolean(method.static),
      });
    }
    for (const ctor of type.constructors || []) {
      contract.constructors.push(contractParameterSpecs(ctor.parameters || ctor));
    }
  }

  for (const doc of data.documents || []) {
    const namespace = doc.namespace || "";
    const name = namespace && namespace !== systemNamespace ? `${namespace}.${doc.name}` : doc.name;
    const contract = ensure(name);
    if (!contract) continue;
    for (const member of doc.members || doc.Members || []) {
      const kind = String(member.kind || "").toLowerCase();
      const type = normalizeContractType(member.propertyType || member.returnType || member.type || "");
      const params = contractParameterSpecs(member.parameters || []);
      if (kind === "property") {
        contract.properties.push({ name: member.name, type, static: Boolean(member.static) });
      } else if (kind === "constructor") {
        if (params.length > 0 || signatureHasEmptyParameterList(member.signature || "")) {
          contract.constructors.push(params);
        }
      } else if (kind === "method") {
        contract.methods.push({ name: member.name, returnType: type, parameters: params, static: Boolean(member.static) });
      }
    }
  }
  return contracts;
}

function contractParameterSpecs(parameters) {
  return (parameters || [])
    .map((param, index) => {
      if (typeof param === "string") return { name: `arg${index}`, type: normalizeContractType(param) };
      return { name: param.name || `arg${index}`, type: normalizeContractType(param.type || param.Type || "") };
    })
    .filter((param) => param.type);
}

function applyDocsContracts(specs, contracts) {
  if (!contracts.size) return specs;
  const byName = new Map(specs.map((spec) => [spec.name.toLowerCase(), spec]));
  for (const contract of contracts.values()) {
    let spec = byName.get(contract.name.toLowerCase());
    if (!spec) {
      spec = emptySpec(contract.name);
      spec.superClass = "";
      specs.push(spec);
      byName.set(spec.name.toLowerCase(), spec);
    }
    for (const contractProp of contract.properties) {
      if (!contractProp.name || !contractProp.type) continue;
      const existing = spec.properties.find((prop) => prop.name.toLowerCase() === contractProp.name.toLowerCase() && Boolean(prop.static) === Boolean(contractProp.static));
      if (existing) {
        if (isUntypedApexType(existing.type)) existing.type = contractProp.type;
        continue;
      }
      spec.properties.push({ name: contractProp.name, type: contractProp.type, static: Boolean(contractProp.static) });
    }
    for (const contractMethod of contract.methods) {
      if (!contractMethod.name || !contractMethod.returnType) continue;
      const existing = findContractMethodMatch(spec.methods, contractMethod);
      if (existing) {
        contractMethod.name = existing.name;
        contractMethod.static = existing.static;
        if (contractMethod.returnType && isUntypedApexType(existing.returnType)) existing.returnType = contractMethod.returnType;
        continue;
      }
      if (isLowInformationContractMethod(contractMethod) &&
        spec.methods.some((method) => method.name.toLowerCase() === contractMethod.name.toLowerCase())) {
        continue;
      }
      if (contractMethod.returnType) spec.methods.push(contractMethod);
    }
    for (const ctor of contract.constructors) {
      mergeContractConstructor(spec, ctor);
    }
    dedupeSpec(spec);
  }
  return specs.sort((a, b) => a.name.localeCompare(b.name));
}

function signatureHasEmptyParameterList(signature) {
  const match = String(signature || "").match(/\(([^)]*)\)/);
  return Boolean(match && match[1].trim() === "");
}

function mergeContractConstructor(spec, ctor) {
  if (spec.constructors.some((existing) => constructorKey(existing) === constructorKey(ctor))) {
    return;
  }
  const weakSameArityIndex = spec.constructors.findIndex((existing) => existing.length === ctor.length && isWeakParameterList(existing));
  if (weakSameArityIndex >= 0 && !isWeakParameterList(ctor)) {
    spec.constructors[weakSameArityIndex] = ctor;
    return;
  }
  if (isWeakParameterList(ctor) && spec.constructors.some((existing) => existing.length === ctor.length)) {
    return;
  }
  spec.constructors.push(ctor);
}

function isUntypedApexType(typeName) {
  const key = normalizeType(typeName).toLowerCase();
  return key === "" || key === "object" || key === "apex_object" || key === "any";
}

function isWeakParameterList(parameters) {
  return parameters.every((param) => isUntypedApexType(param.type) || isGenericPlaceholderType(param.type));
}

function isGenericPlaceholderType(typeName) {
  return /^[A-Z]$/.test(normalizeType(typeName));
}

function ensureProperty(spec, name, type, isStatic = false) {
  if (spec.properties.some((prop) => prop.name === name && Boolean(prop.static) === Boolean(isStatic))) {
    return;
  }
  spec.properties.push({ name, type, static: isStatic });
}

function ensureMethod(spec, name, returnType, parameters = [], isStatic = false) {
  const key = `${name}|${isStatic}|${parameters.map((param) => param.type.toLowerCase()).join(",")}|${returnType.toLowerCase()}`;
  if (spec.methods.some((method) => methodKey(method) === key)) {
    return;
  }
  spec.methods.push({ name, returnType, parameters, static: isStatic });
}

function ensureConstructor(spec) {
  if (!spec.constructors.some((ctor) => ctor.length === 0)) {
    spec.constructors.push([]);
  }
}

function ensureConnectApiStubShape(specs) {
  const byName = new Map(specs.map((spec) => [spec.name.toLowerCase(), spec]));
  const ensureSpec = (name, kind = "", superClass = "Object") => {
    const key = name.toLowerCase();
    if (byName.has(key)) return byName.get(key);
    const spec = emptySpec(name);
    spec.kind = kind;
    spec.superClass = superClass;
    specs.push(spec);
    byName.set(key, spec);
    return spec;
  };

  const nbaFlowAction = ensureSpec("ConnectApi.NBAFlowAction");
  ensureProperty(nbaFlowAction, "parameters", "List<ConnectApi.NBAActionParameter>");

  const event = ensureSpec("ConnectApi.OrchestrationEvent");
  ensureProperty(event, "stageStepInstanceId", "String");
  ensureProperty(event, "workAssignmentId", "String");
  ensureProperty(event, "workStatus", "ConnectApi.OrchestrationWorkStatus");

  const eventInfo = ensureSpec("ConnectApi.OrchestrationEventInfo");
  ensureProperty(eventInfo, "stageStepInstanceId", "String");
  ensureProperty(eventInfo, "workAssignmentId", "String");
  ensureProperty(eventInfo, "workStatus", "ConnectApi.OrchestrationWorkStatus");

  const stage = ensureSpec("ConnectApi.OrchestrationStageInstance");
  ensureProperty(stage, "position", "Object");
  ensureProperty(stage, "stageStepInstances", "List<ConnectApi.OrchestrationStepInstance>");

  const step = ensureSpec("ConnectApi.OrchestrationStepInstance");
  ensureProperty(step, "type", "ConnectApi.OrchestrationStepType");
  ensureProperty(step, "workAssignments", "List<ConnectApi.OrchestrationWorkAssignment>");

  const stepType = ensureSpec("ConnectApi.OrchestrationStepType", "DeclarationEnum");
  ensureProperty(stepType, "Task", "ConnectApi.OrchestrationStepType", true);

  const assignment = ensureSpec("ConnectApi.OrchestrationWorkAssignment");
  ensureConstructor(assignment);
  for (const [name, type] of [
    ["assigneeId", "Object"],
    ["contextRecordId", "Object"],
    ["description", "Object"],
    ["id", "Object"],
    ["label", "Object"],
    ["screenFlowId", "Object"],
    ["screenFlowInputParameters", "Object"],
    ["status", "Object"],
  ]) {
    ensureProperty(assignment, name, type);
  }
  ensureMethod(assignment, "clone", "Object");
  ensureMethod(assignment, "equals", "Boolean", [{ name: "obj", type: "Object" }]);
  ensureMethod(assignment, "getBuildVersion", "Double");
  ensureMethod(assignment, "hashCode", "Integer");
  ensureMethod(assignment, "toString", "String");

  const workStatus = ensureSpec("ConnectApi.OrchestrationWorkStatus", "DeclarationEnum");
  workStatus.constructors = [];
  ensureProperty(workStatus, "FlowCompleted", "ConnectApi.OrchestrationWorkStatus", true);
  ensureMethod(workStatus, "equals", "Boolean", [{ name: "obj", type: "Object" }]);
  ensureMethod(workStatus, "hashCode", "Integer");
  ensureMethod(workStatus, "ordinal", "Integer");
  ensureMethod(workStatus, "valueOf", "ConnectApi.OrchestrationWorkStatus", [{ name: "str", type: "String" }], true);
  ensureMethod(workStatus, "values", "List<ConnectApi.OrchestrationWorkStatus>", [], true);

  for (const spec of specs) {
    dedupeSpec(spec);
  }
  return specs;
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

function parameterName(param, index) {
	param = param.trim();
	if (!param) return `arg${index}`;
	let depth = 0;
	let lastSplit = -1;
	for (let i = 0; i < param.length; i++) {
		const ch = param[i];
		if (ch === "<") depth++;
		if (ch === ">") depth = Math.max(0, depth - 1);
		if (/\s/.test(ch) && depth === 0) {
			lastSplit = i;
		}
	}
	if (lastSplit < 0) return `arg${index}`;
	const name = param.slice(lastSplit).trim();
	return /^[A-Za-z_][A-Za-z0-9_]*$/.test(name) ? name : `arg${index}`;
}

function parameterSpecs(params) {
	if (!params.trim()) return [];
	return splitTopLevel(params)
		.map((param, index) => ({ type: parameterType(param), name: parameterName(param, index) }))
		.filter((param) => param.type);
}

function parameterTypes(params) {
	return parameterSpecs(params).map((param) => param.type);
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
			spec.constructors.push(parameterSpecs(ctor[2]));
      continue;
    }

    const method = line.match(/^\s*(?:global|public)\s+(static\s+)?([A-Za-z_][A-Za-z0-9_.]*(?:<[^;{}()]+>)?)\s+([A-Za-z_][A-Za-z0-9_]*)\s*\(([^)]*)\)/);
    if (method && method[3] !== className) {
      spec.methods.push({
        name: method[3],
        returnType: normalizeType(method[2]),
        parameters: parameterSpecs(method[4]),
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

  const enumFactoryTypes = enumFactoryReturnTypes(spec);
  const looksLikeEnum =
    spec.kind === "DeclarationEnum" ||
    (missingTypeProperties.length > 0 && enumFactoryTypes.length > 0) ||
    (missingTypeProperties.length > 0 &&
      missingTypeProperties.every((prop) => /^[A-Z][A-Z0-9_]*$/.test(prop.name)) &&
      spec.methods.every((method) => ["clone", "equals", "hashCode", "ordinal", "toString"].includes(method.name)));
  inferMissingPropertyShape(spec, missingTypeProperties, looksLikeEnum);
  if (looksLikeEnum) {
    spec.kind = "DeclarationEnum";
    spec.constructors = [];
    for (const prop of missingTypeProperties) {
      prop.type = name;
      prop.static = true;
    }
  }
  for (const prop of spec.properties) {
    delete prop.missingType;
  }

  applySignatureOverrides(spec);
  dedupeSpec(spec);
  return spec;
}

function inferMissingPropertyShape(spec, missingTypeProperties, looksLikeEnum) {
  const zeroArgMethods = new Map();
  const constructorParamTypes = new Map();
  const setterParamTypes = new Map();
  function addInferredType(target, name, type, isStatic = false) {
    const key = name.toLowerCase();
    const existing = target.get(key);
    if (existing && (existing.type !== type || existing.static !== isStatic)) {
      target.set(key, { conflict: true });
      return;
    }
    target.set(key, { type, static: isStatic });
  }
  for (const params of spec.constructors) {
    for (const param of params) {
      if (param.name && !param.name.startsWith("arg") && param.type) {
        addInferredType(constructorParamTypes, param.name, param.type, false);
      }
    }
  }
  for (const method of spec.methods) {
    if (method.parameters.length === 0 && method.returnType) {
      zeroArgMethods.set(method.name.toLowerCase(), method);
      continue;
    }
    if (method.parameters.length === 1 && method.name.toLowerCase().startsWith("set")) {
      const suffix = method.name.slice(3);
      if (suffix) {
        addInferredType(setterParamTypes, suffix, method.parameters[0].type, method.static);
      }
    }
  }
  for (const prop of missingTypeProperties) {
    const getterKey = `get${capitalizeIdentifier(prop.name)}`.toLowerCase();
    const booleanGetterKey = `is${capitalizeIdentifier(prop.name)}`.toLowerCase();
    const inferredGetter = zeroArgMethods.get(getterKey);
    const inferredBooleanGetter = zeroArgMethods.get(booleanGetterKey);
    const inferredBooleanNamedGetter = booleanPropertyName(prop.name) ? zeroArgMethods.get(prop.name.toLowerCase()) : undefined;
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
    if (inferredBooleanNamedGetter) {
      prop.type = inferredBooleanNamedGetter.returnType;
      prop.static = inferredBooleanNamedGetter.static;
      continue;
    }
    if (looksLikeEnum) {
      continue;
    }
    const inferredConstructorParam = constructorParamTypes.get(prop.name.toLowerCase());
    if (inferredConstructorParam && !inferredConstructorParam.conflict) {
      prop.type = inferredConstructorParam.type;
      prop.static = inferredConstructorParam.static;
      continue;
    }
    const inferredSetterParam = setterParamTypes.get(prop.name.toLowerCase());
    if (inferredSetterParam && !inferredSetterParam.conflict) {
      prop.type = inferredSetterParam.type;
      prop.static = inferredSetterParam.static;
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

function booleanPropertyName(value) {
  return /^is[A-Z0-9_]/.test(value || "");
}

function constructorKey(params) {
  return params.map((p) => p.type.toLowerCase()).join(",");
}

function parameterTypesKey(params) {
  return params.map((p) => p.type.toLowerCase()).join(",");
}

function findContractMethodMatch(methods, contractMethod) {
  const wantedName = contractMethod.name.toLowerCase();
  const wantedParameters = parameterTypesKey(contractMethod.parameters);
  return methods.find((method) =>
    method.name.toLowerCase() === wantedName &&
      Boolean(method.static) === Boolean(contractMethod.static) &&
      parameterTypesKey(method.parameters) === wantedParameters) ||
    methods.find((method) =>
      method.name.toLowerCase() === wantedName &&
      parameterTypesKey(method.parameters) === wantedParameters) ||
    methods.find((method) =>
      method.name.toLowerCase() === wantedName &&
      method.parameters.length === contractMethod.parameters.length &&
      contractMethod.parameters.some((param) => isGenericPlaceholderType(param.type)));
}

function isLowInformationContractMethod(method) {
  return (!method.returnType || isUntypedApexType(method.returnType)) && method.parameters.length === 0;
}

function methodKey(method) {
  return `${method.name.toLowerCase()}|${method.static}|${parameterTypesKey(method.parameters)}|${method.returnType.toLowerCase()}`;
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
    out += "\t\tConstructorSpecs: []StandardConstructorSpec{\n";
    for (const ctor of spec.constructors) {
      out += `\t\t\t{Parameters: []StandardParameterSpec{${ctor.map((param) => `{Name: ${goString(param.name)}, Type: ${goString(param.type)}}`).join(", ")}},},\n`;
    }
    out += "\t\t},\n";
  }
  if (spec.methods.length) {
    out += "\t\tMethods: []StandardMethodSpec{\n";
    for (const method of spec.methods) {
      out += `\t\t\t{Name: ${goString(method.name)}, ReturnType: ${goString(method.returnType)}`;
      if (method.parameters.length) out += `, ParameterSpecs: []StandardParameterSpec{${method.parameters.map((param) => `{Name: ${goString(param.name)}, Type: ${goString(param.type)}}`).join(", ")}}`;
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

function splitTypeName(name) {
  const idx = String(name || "").lastIndexOf(".");
  if (idx <= 0 || idx >= name.length - 1) return ["", name || ""];
  return [name.slice(0, idx), name.slice(idx + 1)];
}

function emptySpec(name) {
  return { name, kind: "", superClass: "Object", interfaces: [], constructors: [], methods: [], properties: [] };
}

function cloneMethod(method, overrides = {}) {
  return {
    name: overrides.name ?? method.name,
    returnType: overrides.returnType ?? method.returnType,
    parameters: method.parameters.map((param) => ({ name: param.name, type: param.type })),
    static: overrides.static ?? method.static,
  };
}

function materializeNestedStubSpecs(specs) {
  const byName = new Map(specs.map((spec) => [spec.name.toLowerCase(), spec]));
  const additions = new Map();
  const getAddition = (name) => {
    const key = name.toLowerCase();
    if (byName.has(key)) return byName.get(key);
    if (!additions.has(key)) additions.set(key, emptySpec(name));
    return additions.get(key);
  };

  for (const spec of specs) {
    const [, localName] = splitTypeName(spec.name);
    if (localName === "Builder") {
      for (const method of spec.methods) {
        if (method.returnType.endsWith(".Builder")) {
          const builder = getAddition(method.returnType);
          builder.constructors.push([]);
          builder.methods.push(cloneMethod(method, { static: false }));
        } else if (method.name === "build" && method.returnType && !isBuiltinTypeName(method.returnType)) {
          const builder = getAddition(`${method.returnType}.Builder`);
          builder.constructors.push([]);
          builder.methods.push(cloneMethod(method, { static: false }));
        }
      }
      continue;
    }

    for (const enumAlias of nestedEnumAliasNames(spec)) {
      const alias = getAddition(enumAlias);
      alias.kind = "DeclarationEnum";
      alias.superClass = spec.superClass || "Object";
      alias.constructors = [];
      alias.methods.push(...enumAliasMethods(spec, alias.name));
      alias.properties.push(...spec.properties.map((prop) => ({
        name: prop.name,
        type: prop.static && (prop.type === "Object" || prop.type === spec.name) ? alias.name : prop.type,
        static: true,
      })));
    }
  }

  for (const spec of additions.values()) {
    if (spec.constructors.length === 0 && spec.kind !== "DeclarationEnum") {
      spec.constructors.push([]);
    }
    if (spec.kind !== "DeclarationEnum" && !spec.methods.some((method) => method.name === "clone")) {
      spec.methods.push({ name: "clone", returnType: "Object", parameters: [], static: false });
    }
    dedupeSpec(spec);
  }

  return specs.concat([...additions.values()]).sort((a, b) => a.name.localeCompare(b.name));
}

function enumFactoryReturnTypes(spec) {
  const out = [];
  for (const valueOf of spec.methods) {
    if (!valueOf.static || valueOf.name !== "valueOf" || !valueOf.returnType) {
      continue;
    }
    if (!spec.methods.some((method) => method.static && method.name === "values" && method.returnType === `List<${valueOf.returnType}>`)) {
      continue;
    }
    out.push(valueOf.returnType);
  }
  return [...new Set(out)].sort();
}

function nestedEnumAliasNames(spec) {
	if (!spec.properties.some((prop) => prop.static && prop.name)) return [];
	return enumFactoryReturnTypes(spec).filter((typeName) => typeName !== spec.name);
}

function enumAliasMethods(spec, aliasName) {
  const out = [];
  for (const method of spec.methods) {
    if (method.static && method.name === "valueOf") {
      if (method.returnType === aliasName) out.push(cloneMethod(method, { returnType: aliasName }));
      continue;
    }
    if (method.static && method.name === "values") {
      if (method.returnType === `List<${aliasName}>`) out.push(cloneMethod(method, { returnType: `List<${aliasName}>` }));
      continue;
    }
    out.push(cloneMethod(method, {
      returnType: method.returnType === spec.name ? aliasName : method.returnType.replaceAll(spec.name, aliasName),
    }));
  }
  return out;
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
      for (const param of ctor) consider(param.type);
    }
    for (const method of spec.methods) {
      consider(method.returnType);
      for (const param of method.parameters) consider(param.type);
    }
    for (const prop of spec.properties) consider(prop.type);
  }
  return specs.concat([...additions.values()]).sort((a, b) => a.name.localeCompare(b.name));
}

function applyGeneratedCompatibilityOverrides(specs) {
  const byName = new Map(specs.map((spec) => [spec.name.toLowerCase(), spec]));
  forcePropertyType(byName, "RestResponse", "responseBody", false, "Object");
  forcePropertyType(byName, "ConnectApi.OrchestrationStageInstance", "status", false, "ConnectApi.OrchestrationInstanceStatus");
  forcePropertyType(byName, "ConnectApi.OrchestrationStepInstance", "status", false, "ConnectApi.OrchestrationInstanceStatus");
  forcePropertyType(byName, "ConnectApi.QuestionAndAnswersSuggestions", "articles", false, "Object");
  forcePropertyType(byName, "ConnectApi.QuestionAndAnswersSuggestions", "questions", false, "Object");
  for (const spec of specs) dedupeSpec(spec);
  return specs;
}

function forcePropertyType(byName, specName, propertyName, isStatic, typeName) {
  const spec = byName.get(specName.toLowerCase());
  if (!spec) return;
  const prop = spec.properties.find((candidate) =>
    candidate.name.toLowerCase() === propertyName.toLowerCase() &&
      Boolean(candidate.static) === Boolean(isStatic));
  if (prop) {
    if (!isUntypedApexType(prop.type)) return;
    prop.type = typeName;
    return;
  }
  spec.properties.push({ name: propertyName, type: typeName, static: Boolean(isStatic) });
}

const docsContracts = loadDocsContracts(docsContractFile);
const specs = addReferencedPlaceholders(applyGeneratedCompatibilityOverrides(applyDocsContracts(materializeNestedStubSpecs(ensureConnectApiStubShape(collectStubFiles(inputRoot)
  .map(parseStub)
  .filter(Boolean)
  .sort((a, b) => a.name.localeCompare(b.name)))), docsContracts)));

let out = `// Code generated by scripts/generate-system-stub-symbols.mjs; DO NOT EDIT.\n\n`;
out += `package typesys\n\n`;
out += `import "github.com/glade-sh/glade/internal/apexast"\n\n`;
out += `var systemStubSymbolSpecs = []StandardSymbolSpec{\n`;
for (const spec of specs) out += writeSpec(spec);
out += `}\n`;

fs.writeFileSync(outputFile, out);
const gofmt = spawnSync("gofmt", ["-w", outputFile], { stdio: "inherit" });
if (gofmt.status !== 0) process.exit(gofmt.status ?? 1);
