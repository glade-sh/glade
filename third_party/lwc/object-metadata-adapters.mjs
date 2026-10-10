// The UI object metadata adapters are wire-only. getLayout remains usable as
// an ordinary binding, as captured alongside these adapters at APIs 59 and 67.
const wireOnlyAdapters = new Map([
  ["lightning/uiObjectInfoApi", new Set([
    "getObjectInfo", "getObjectInfos", "getPicklistValues", "getPicklistValuesByRecordType",
  ])],
  ["lightning/uiRecordApi", new Set(["getRecordCreateDefaults"])],
]);

export function validateObjectMetadataAdapters(ast, traverse) {
  if (!ast.program.body.some((node) => node.type === "ImportDeclaration" && wireOnlyAdapters.has(node.source.value))) {
    return;
  }
  traverse(ast, {
    Program(program) {
      for (const statement of program.node.body) {
        if (statement.type !== "ImportDeclaration") continue;
        const adapters = wireOnlyAdapters.get(statement.source.value);
        if (!adapters) continue;
        for (const imported of statement.specifiers) {
          if (imported.type !== "ImportSpecifier" || !adapters.has(imported.imported.name ?? imported.imported.value)) continue;
          const binding = program.scope.getBinding(imported.local.name);
          for (const reference of binding.referencePaths) {
            if (isWireDecoratorArgument(reference)) continue;
            const { line, column } = reference.node.loc.start;
            const error = new Error(`[Line: ${line}, Col: ${column + 1}] LWC1503: "${imported.local.name}" is a wire adapter and can only be used via the @wire decorator.`);
            error.code = 1503;
            throw error;
          }
        }
      }
      program.stop();
    },
  });
}

function isWireDecoratorArgument(reference) {
  const call = reference.parentPath;
  if (!call.isCallExpression() || call.node.arguments[0] !== reference.node || !call.parentPath.isDecorator()) return false;
  const callee = call.get("callee");
  if (!callee.isIdentifier()) return false;
  const binding = callee.scope.getBinding(callee.node.name);
  return binding?.path.isImportSpecifier() &&
    (binding.path.node.imported.name ?? binding.path.node.imported.value) === "wire" &&
    binding.path.parent.source.value === "lwc";
}
