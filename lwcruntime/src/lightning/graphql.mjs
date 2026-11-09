import { parseDocument, opaqueDocument, isOpaqueDocument, unwrapDocument } from "./graphql-document.mjs";
import { executeQuery, NO_WIRE_PROVISION } from "./graphql-query.mjs";

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
  let document, parseError;
  try { document = parseDocument(source); } catch (error) { parseError = error; }
  if (document) {
    const unsupported = document.definitions.filter(node => node.kind === "OperationDefinition" && node.operation !== "query" && node.operation !== "mutation");
    if (unsupported.length) throw new SyntaxError("There are unsupported operations in the graphql query:\n" + unsupported.map(node => `Unsupported ${node.operation} (${node.name || "anonymous"}) operation.`).join("\n"));
    document.loc = { source: { body: source, name: "GraphQL request" } };
    document.toString = () => source;
  }
  const value = opaqueDocument(document, source, parseError);
  documents.set(source, value);
  return value;
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
  let parsed;
  try { parsed = unwrapDocument(input?.query); }
  catch (error) { return Promise.resolve({ data: undefined, errors: [{ errorType: "localBoundary", message: error.message }] }); }
  if (parsed?.kind === "Document" && typeof window !== "undefined") return executeQuery(parsed, input.variables, input.operationName);
  return Promise.resolve({ data: readGraphQLData(), errors: [] });
}
graphql.prototype.connect = function () { this.connected = true; if (this.config) this.update(this.config); };
graphql.prototype.disconnect = function () { this.connected = false; this.generation++; };
graphql.prototype.update = function (config) {
  this.config = config;
  if (!this.connected) return;
  let parsed;
  try { parsed = unwrapDocument(config?.query); }
  catch (error) {
    this.generation++;
    publish(this, { data: undefined, errors: [{ errorType: "localBoundary", message: error.message }] });
    return;
  }
  if (parsed?.kind !== "Document" || (Object.prototype.hasOwnProperty.call(config, "variables") && config.variables === undefined)) {
    this.generation++; return;
  }
  const generation = ++this.generation;
  this.pending = executeQuery(parsed, config.variables ?? {}, config.operationName, true).then(value => {
    if (!this.connected || generation !== this.generation || value === NO_WIRE_PROVISION) return;
    publish(this, value);
  }).catch(error => {
    if (!this.connected || generation !== this.generation) return;
    const value = { data: undefined, errors: [{ errorType: "localBoundary", message: error.message }] };
    publish(this, value);
  });
};
// Invalid inputs match the captured desktop adapter envelope. Mutation execution
// itself remains a hosted boundary and is not simulated as a successful write.
export function executeMutation(config) {
  if (!isOpaqueDocument(config?.query) && config?.query?.kind !== "Document") {
    return Promise.resolve({ errors: [{ message: "Internal error in GraphQL adapter occurred", locations: [] }] });
  }
  return Promise.resolve({ errors: [{ errorType: "localBoundary", message: "GraphQL mutation execution is not supported locally" }] });
}
export function refreshGraphQL(value) {
  const adapter = value && typeof value === "object" ? provisions.get(value) || value[provision]?.() : null;
  if (!adapter?.config) return Promise.resolve();
  // Refresh settles without emitting an unchanged cached result (native
  // r_envelope_refresh and r_pages_refresh_page). Changed data is provisioned.
  const generation = adapter.generation;
  return executeQuery(adapter.config.query, adapter.config.variables ?? {}, adapter.config.operationName, true).then(next => {
    if (next !== NO_WIRE_PROVISION && generation === adapter.generation && JSON.stringify(next) !== JSON.stringify(adapter.value) && adapter.connected) {
      publish(adapter, next);
    }
  });
}
export function query(documentOrConfig, variables = {}) {
  return graphql(documentOrConfig?.query ? documentOrConfig : { query: documentOrConfig, variables });
}
export default graphql;
