import { parseDocument } from "../../lwcruntime/src/lightning/graphql-document.mjs";

// Native deployment checks fields in static UI GraphQL documents against the
// project schema. Invalid document syntax remains a gql construction error.
export function validateGraphQLDocuments(ast, schema, walkAST) {
  const tags = new Set();
  for (const statement of ast.program.body) {
    if (statement.type !== "ImportDeclaration" || !["lightning/uiGraphQLApi", "lightning/graphql"].includes(statement.source.value)) continue;
    for (const binding of statement.specifiers) {
      if (binding.type === "ImportSpecifier" && binding.imported.name === "gql") tags.add(binding.local.name);
    }
  }
  if (!tags.size) return;
  walkAST(ast, node => {
    if (node.type !== "TaggedTemplateExpression" || node.tag.type !== "Identifier" || !tags.has(node.tag.name) || node.quasi.expressions.length) return;
    const source = node.quasi.quasis.map(part => part.value.cooked ?? part.value.raw).join("");
    let document;
    try { document = parseDocument(source); }
    catch { return; }
    const fragments = new Map(document.definitions.filter(definition => definition.kind === "FragmentDefinition").map(definition => [definition.name, definition]));
    function selections(nodes, visit, trail = []) {
      for (const field of nodes) {
        if (field.kind === "Field") visit(field);
        else if (field.kind === "InlineFragment") selections(field.selection, visit, trail);
        else if (!trail.includes(field.name) && fragments.has(field.name)) selections(fragments.get(field.name).selection, visit, [...trail, field.name]);
      }
    }
    function recordFields(nodes, objectName, path) {
      const object = schema?.[objectName];
      if (!object) return;
      selections(nodes, field => {
        if (field.name === "__typename") return;
        const current = [...path, field.name];
        const references = object.relationships?.[field.name];
        if (!object.fields[field.name] && !references) {
          const prefix = source.slice(0, field.start);
          const lines = prefix.split("\n");
          throw new Error(`Validation error (FieldUndefined@[${current.join("/")}]) : Field '${field.name}' in type '${objectName}' is undefined. line: ${lines.length}, column: ${lines.at(-1).length + 1}, startIndex: ${field.start + 1}`);
        }
        if (references?.length === 1) recordFields(field.selection, references[0], current);
      });
    }
    for (const operation of document.definitions.filter(definition => definition.kind === "OperationDefinition")) {
      selections(operation.selection, root => {
        if (root.name !== "uiapi") return;
        selections(root.selection, query => {
          if (query.name !== "query") return;
          selections(query.selection, connection => selections(connection.selection, edges => {
            if (edges.name !== "edges") return;
            selections(edges.selection, record => {
              if (record.name === "node") recordFields(record.selection, connection.name, ["uiapi", "query", connection.name, "edges", "node"]);
            });
          }));
        });
      });
    }
  });
}
