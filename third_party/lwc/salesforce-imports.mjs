// Salesforce virtual-module validation, separate from bundle/template grammar.
// API 59/67 owned captures establish binding, identifier and reference errors.
const providers = new Set([
  "schema", "label", "resourceUrl", "contentAssetUrl", "user", "userPermission",
  "customPermission", "i18n", "client", "community", "site",
]);
const identifiers = {
  user: new Set(["Id", "isGuest"]),
  client: new Set(["formFactor"]),
  community: new Set(["Id", "basePath"]),
  site: new Set(["Id", "activeLanguages"]),
};

// Babel's StringLiteral.value is decoded, including escaped provider/object
// names. Hydration and validation must use this same decoded module reference.
export function salesforceImportSpecifier(node) {
  if ((node.type === "ImportDeclaration" || node.type === "ExportNamedDeclaration" || node.type === "ExportAllDeclaration") && node.source?.type === "StringLiteral") {
    return node.source.value;
  }
  if (node.type === "CallExpression" && node.callee?.type === "Import" && node.arguments?.[0]?.type === "StringLiteral") {
    return node.arguments[0].value;
  }
  if (node.type === "ImportExpression" && node.source?.type === "StringLiteral") {
    return node.source.value;
  }
  return null;
}

export function validateSalesforceImport(node, specifier, filename, metadata) {
  const parts = specifier.split("/");
  const provider = parts[1];
  const owned = [...providers].find(name => name.toLowerCase() === provider?.toLowerCase());
  if (parts[0].toLowerCase() !== "@salesforce" || !owned) return;
  const file = filename.split(/[\\/]/).at(-1);
  if (parts[0] !== "@salesforce" || provider !== owned) {
    throw new Error(`Exception while getting the LWC bundle for reference ${specifier} of type module in file ${file}`);
  }
  // Native resourceUrl LWC1704/1705 diagnostics point at the module literal,
  // using Babel's actual (zero-based) column. Keep the underlying text intact.
  const moduleError = message => {
    const error = new Error(message);
    if (provider === "resourceUrl") error.location = node.source?.loc?.start;
    throw error;
  };
  if (node.type === "ImportDeclaration" && node.specifiers.some(binding => binding.type !== "ImportDefaultSpecifier")) {
    moduleError(`LWC1705: @salesforce/${provider} modules only support default imports.`);
  }
  const resource = parts.slice(2).join("/");
  if (parts.length > 2 && identifiers[provider] && !identifiers[provider].has(resource)) {
    throw new Error(`LWC1703: Invalid module identifier "${specifier}".`);
  }
  if (!resource) {
    moduleError(`LWC1704: Missing resource value for ${specifier}`);
  }
  if (!metadata) return; // Older direct compiler callers have no metadata registry.
  const invalid = type => { throw new Error(`Invalid reference ${resource} of type ${type} in file ${file}`); };
  switch (provider) {
    case "schema": {
      const path = resource.split(".");
      const objectFor = name => Object.hasOwn(metadata.schema, name) ? metadata.schema[name] : undefined;
      let objects = [objectFor(path[0])].filter(Boolean);
      if (!objects.length) invalid(path.length === 1 ? "sobjectClass" : "sobjectField");
      // Salesforce schema imports support up to three parent relationships.
      // https://developer.salesforce.com/docs/platform/lwc/guide/reference-salesforce-modules.html
      if (path.length > 5) invalid("sobjectField");
      for (const relationship of path.slice(1, -1)) {
        objects = objects.flatMap(object => Object.hasOwn(object.relationships, relationship) ? object.relationships[relationship] : [])
          .map(objectFor).filter(Boolean);
        if (!objects.length) invalid("sobjectClass");
      }
      if (path.length > 1 && !objects.some(object => Object.hasOwn(object.fields, path.at(-1)))) invalid("sobjectField");
      break;
    }
    case "label":
      if (!resource.includes(".")) throw new Error(`Labels should have a section and a name: ${resource}`);
      if (!Object.hasOwn(metadata.labels, resource)) invalid("label");
      break;
    case "resourceUrl":
      if (!Object.hasOwn(metadata.resources, resource)) invalid("resourceUrl");
      break;
    case "contentAssetUrl":
      if (!Object.hasOwn(metadata.assets, resource)) throw new Error(`Exception while getting the content asset for reference ${resource} of type contentAssetUrl in file ${file}`);
      break;
    case "customPermission":
      if (!Object.hasOwn(metadata.customPermissions, resource)) invalid("customPermission");
      break;
  }
}
