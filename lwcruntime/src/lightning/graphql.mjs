import { parseDocument } from "./graphql-document.mjs";
import { executeQuery } from "./graphql-query.mjs";

const documents = new Map();
const provisions = new WeakMap();
const provision = Symbol("GraphQL provision");
function publish(adapter, value) {
  // A non-enumerable symbol survives LWC's reactive membrane without adding
  // refresh or error fields to the native {data, errors} envelope.
  Object.defineProperty(value, provision, { value: () => adapter });
  provisions.set(value, adapter);
  adapter.value = value;
  adapter.callback(value);
}
function sourceFromTemplate(strings, values) {
  if (Array.isArray(strings) && Object.prototype.hasOwnProperty.call(strings, "raw")) {
    return strings.reduce((out, chunk, index) => out + chunk + (index < values.length ? String(values[index] ?? "") : ""), "");
  }
  return String(strings || "");
}

export function gql(strings, ...values) {
  const source = sourceFromTemplate(strings, values);
  if (!source.trim()) return undefined;
  if (documents.has(source)) return documents.get(source);
  const document = parseDocument(source);
  const unsupported = document.definitions.filter(node => node.kind === "OperationDefinition" && node.operation !== "query");
  if (unsupported.length) throw new SyntaxError("There are unsupported operations in the graphql query:\n" + unsupported.map(node => `Unsupported ${node.operation} (${node.name || "anonymous"}) operation.`).join("\n"));
  document.loc = { source: { body: source, name: "GraphQL request" } };
  document.toString = () => source;
  documents.set(source, document);
  return document;
}

function readGraphQLData() {
  if (typeof document === "undefined") return {};
  const node = document.getElementById("glade-lwc-graphql");
  if (!node) return {};
  try { const payload = JSON.parse(node.textContent || "{}"); return payload && typeof payload.data === "object" ? payload.data : {}; }
  catch { return {}; }
}

// LWC constructs adapters with a callback. Imperative local callers retain the
// existing envelope API; only valid documents enter the query executor.
export function graphql(input) {
  if (typeof input === "function") {
    this.callback = input;
    this.config = null;
    this.generation = 0;
    this.connected = false;
    return;
  }
  if (input?.query?.kind === "Document" && typeof window !== "undefined") return executeQuery(input.query, input.variables, input.operationName);
  return Promise.resolve({ data: readGraphQLData(), errors: [] });
}
graphql.prototype.connect = function () { this.connected = true; if (this.config) this.update(this.config); };
graphql.prototype.disconnect = function () { this.connected = false; this.generation++; };
graphql.prototype.update = function (config) {
  this.config = config;
  if (!this.connected) return;
  if (config?.query?.kind !== "Document" || (Object.prototype.hasOwnProperty.call(config, "variables") && config.variables === undefined)) {
    this.generation++; return;
  }
  const generation = ++this.generation;
  this.pending = executeQuery(config.query, config.variables ?? {}, config.operationName).then(value => {
    if (!this.connected || generation !== this.generation) return;
    publish(this, value);
  }).catch(error => {
    if (!this.connected || generation !== this.generation) return;
    const value = { data: undefined, errors: [{ errorType: "localBoundary", message: error.message }] };
    publish(this, value);
  });
};
export function refreshGraphQL(value) {
  const adapter = value && typeof value === "object" ? provisions.get(value) || value[provision]?.() : null;
  if (!adapter?.config) return Promise.resolve();
  // Refresh settles without emitting an unchanged cached result (native
  // r_envelope_refresh and r_pages_refresh_page). Changed data is provisioned.
  const generation = adapter.generation;
  return executeQuery(adapter.config.query, adapter.config.variables ?? {}, adapter.config.operationName).then(next => {
    if (generation === adapter.generation && JSON.stringify(next) !== JSON.stringify(adapter.value) && adapter.connected) {
      publish(adapter, next);
    }
  });
}
export function query(documentOrConfig, variables = {}) {
  return graphql(documentOrConfig?.query ? documentOrConfig : { query: documentOrConfig, variables });
}
export default graphql;
