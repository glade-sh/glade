import { parseDocument, unwrapDocument } from "./graphql-document.mjs";
import { gql as modernGql } from "./graphql.mjs";
export { default, graphql, query, refreshGraphQL } from "./graphql.mjs";

const legacyDocuments = new Map();

// The legacy wire module retains its captured query-only document contract.
export function gql(strings, ...values) {
  const parsed = unwrapDocument(modernGql(strings, ...values));
  if (!parsed) return parsed;
  const source = parsed.source;
  if (legacyDocuments.has(source)) return legacyDocuments.get(source);
  const document = parseDocument(source);
  if (document) {
    document.loc = { source: { ...parsed.loc.source } };
    document.toString = () => source;
  }
  const unsupported = document?.definitions?.filter(node => node.kind === "OperationDefinition" && node.operation !== "query") || [];
  if (unsupported.length) throw new SyntaxError("There are unsupported operations in the graphql query:\n" + unsupported.map(node => `Unsupported ${node.operation} (${node.name || "anonymous"}) operation.`).join("\n"));
  legacyDocuments.set(source, document);
  return document;
}
