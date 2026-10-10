import { parseDocument, printDocument, resolvedValue } from "./graphql-document.mjs";

const rest = "/services/data/v67.0";
const schemas = new Map();
async function jsonRequest(url) {
  const response = await fetch(url, { credentials: "same-origin" });
  const body = await response.json();
  if (!response.ok) {
    const error = new Error((Array.isArray(body) ? body[0]?.message : body.message) || response.statusText);
    error.status = response.status;
    error.url = url;
    throw error;
  }
  return body;
}
async function describe(name) {
  if (!schemas.has(name)) schemas.set(name, jsonRequest(rest + "/sobjects/" + encodeURIComponent(name) + "/describe"));
  return schemas.get(name);
}
const scalarTypes = { boolean: "Boolean", date: "Date", datetime: "DateTime", currency: "Currency", double: "Double", int: "Int", reference: "ID", id: "ID" };
const numericTypes = new Set(["currency", "double", "decimal", "int"]);
const fieldType = metadata => String(metadata?.type || "").toLowerCase();
const keyFor = node => node.alias || node.name;
const argumentsFor = (node, variables) => Object.fromEntries(node.args.map(arg => [arg.name, resolvedValue(arg.value, variables)]));
const location = (source, node) => {
  const prefix = source.slice(0, node.start);
  const lines = prefix.split("\n");
  return { line: lines.length, column: lines.at(-1).length + 1 };
};
function included(node, variables) {
  return node.directives.every(d => {
    const flag = argumentsFor(d, variables).if;
    return d.name === "skip" ? !flag : d.name === "include" ? !!flag : true;
  });
}
function merged(left, right) {
  if (Array.isArray(left) && Array.isArray(right)) return left.map((item, i) => merged(item, right[i]));
  if (left && right && typeof left === "object" && typeof right === "object") {
    const out = { ...left };
    for (const [key, value] of Object.entries(right)) out[key] = key in out ? merged(out[key], value) : value;
    return out;
  }
  return right;
}
function javaValue(value) {
  if (value === null) return "NullValue{}";
  if (typeof value === "string") return `StringValue{value='${value}'}`;
  if (typeof value === "boolean") return `BooleanValue{value=${value}}`;
  if (typeof value === "number") return `${Number.isInteger(value) ? "Int" : "Float"}Value{value=${value}}`;
  if (value?.kind === "Variable") return `VariableReference{name='${value.name}'}`;
  if (value?.kind === "Enum") return `EnumValue{name='${value.name}'}`;
  if (Array.isArray(value)) return "ArrayValue{values=[" + value.map(javaValue).join(", ") + "]}";
  return "ObjectValue{objectFields=[" + Object.entries(value).map(([name, val]) => `ObjectField{name='${name}', value=${javaValue(val)}}`).join(", ") + "]}";
}
function javaString(value) {
  if (Array.isArray(value)) return "[" + value.map(javaString).join(", ") + "]";
  if (value && typeof value === "object") return "{" + Object.entries(value).map(([key, val]) => key + "=" + javaString(val)).join(", ") + "}";
  return String(value);
}
function connectionReference(path, args) {
  function argumentKey(value) {
    if (Array.isArray(value)) return [...value].sort().map((item, index) => argumentKey(item) + "[" + index + "]").join(",");
    if (value && typeof value === "object") return Object.keys(value).sort().map(key => key + ":" + argumentKey(value[key])).join(":");
    return String(value);
  }
  const arguments_ = Object.keys(args).sort().map(key => key + ":" + argumentKey(args[key])).join("::");
  return { __ref: `UiApi::${path[0]}::Query[${path[0]}]__${path.join("__")}__args__(${arguments_})` };
}
function matches(record, filter) {
  if (!filter) return true;
  return Object.entries(filter).every(([field, operators]) => {
    if (field === "and") return operators.every(item => matches(record, item));
    if (field === "or") return !operators.length || operators.some(item => matches(record, item));
    if (field === "not") return !Object.keys(operators).length || !matches(record, operators);
    const value = record[field] ?? null;
    return Object.entries(operators || {}).every(([op, expected]) => {
      // UI API string inputs coerce non-string variable values before filtering.
      const comparable = expected === "" ? null : typeof value === "string" && expected !== null ? String(expected) : expected;
      if (op === "eq") return value === comparable;
      if (op === "ne") return value !== comparable;
      if (op === "in") return expected.some(item => matches(record, { [field]: { eq: item } }));
      if (op === "nin") return !expected.some(item => matches(record, { [field]: { eq: item } }));
      if (op === "like") {
        if (value === null) return false;
        let pattern = "", escaped = false;
        for (const character of String(expected)) {
          if (!escaped && character === "\\") { escaped = true; continue; }
          pattern += !escaped && character === "%" ? ".*" : !escaped && character === "_" ? "." : character.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
          escaped = false;
        }
        if (escaped) pattern += "\\\\";
        return new RegExp("^" + pattern + "$", "iu").test(value);
      }
      return false;
    });
  });
}

// Execute against the product's schema and SOQL endpoints, never supplied
// answers or fixture-name branches. AST aliases and fragment projections share
// the same resolver as direct selections.
export async function executeQuery(input, suppliedVariables = {}, operationName) {
  const source = printDocument(input);
  const document = parseDocument(source);
  const errors = [];
  const errorKeys = new Set();
  function error(message, nodes = [], errorType = "ValidationError", paths = []) {
    const value = { extensions: errorType ? { ErrorType: errorType } : {}, locations: nodes.map(node => location(source, node)), message, paths };
    const key = JSON.stringify(value);
    // Schema validation is independent of the number of resolved records.
    if (!errorKeys.has(key)) { errors.push(value); errorKeys.add(key); }
  }
  const validation = (code, path, detail, nodes) => error(`Validation error (${code}${path.length ? "@[" + path.join("/") + "]" : ""}) : ${detail}`, nodes);
  const operations = document.definitions.filter(node => node.kind === "OperationDefinition");
  const hasSelection = (nodes, predicate) => nodes.some(node => predicate(node) || (node.selection && hasSelection(node.selection, predicate)));
  // The UI GraphQL transport explicitly refuses introspection; it is not an
  // empty local schema result. API 59/67 capture the same response body.
  if (document.definitions.some(node => hasSelection(node.selection, field => field.name === "__schema" || field.name === "__type"))) {
    return { data: undefined, errors: [{ body: { errorCode: "ILLEGAL_QUERY_PARAMETER_VALUE", id: "-1822376259", message: "Introspection queries are disabled in Aura.", statusCode: 400 }, errorType: "fetchResponse", headers: {}, ok: false, status: 400, statusText: "Bad Request" }] };
  }
  const fragments = new Map();
  const operationNames = new Set();
  for (const node of document.definitions) {
    if (node.kind === "FragmentDefinition") {
      if (fragments.has(node.name)) validation("DuplicateFragmentName", [node.name], `There can be only one fragment named '${node.name}'`, [node]);
      fragments.set(node.name, node);
    } else if (node.name) {
      if (operationNames.has(node.name)) validation("DuplicateOperationName", [], `There can be only one operation named '${node.name}'`, [node]);
      operationNames.add(node.name);
    }
  }
  let operation = operationName ? operations.find(node => node.name === operationName) : operations.length === 1 ? operations[0] : null;
  if (!operation) {
    error(operationName ? `Unknown operation named '${operationName}'.` : "Must provide operation name if query contains multiple operations.", [], "");
    return { data: undefined, errors: [{ error: errors, errorType: "adapterError" }] };
  }
  const variables = {};
  for (const variable of operation.variables) {
    let value = suppliedVariables?.[variable.name];
    if (value == null && variable.defaultValue !== undefined) value = variable.defaultValue;
    value = resolvedValue(value, suppliedVariables || {});
    if (value == null && variable.type.endsWith("!")) {
      error(`Variable '${variable.name}' has an invalid value: Variable '${variable.name}' has coerced Null value for NonNull type '${variable.type}'`, [variable]);
    } else if (value != null && variable.type.replace(/!$/, "") === "Int") {
      const converted = typeof value === "string" && /^-?\d+$/.test(value) ? Number(value) : value;
      if (typeof converted !== "number" || !Number.isInteger(converted)) {
        const type = Array.isArray(value) ? "ArrayList" : typeof value === "object" ? "LinkedHashMap" : typeof value === "boolean" ? "Boolean" : typeof value === "number" ? "Double" : "String";
        error(`Variable '${variable.name}' has an invalid value: Expected a value that can be converted to type 'Int' but it was a '${type}'`, [variable]);
      }
      value = converted;
    } else if (value != null && variable.type.replace(/!$/, "") === "Boolean" && typeof value === "string") value = value === "true";
    else if (value != null && variable.type.replace(/!$/, "") === "String") value = javaString(value);
    variables[variable.name] = value;
  }
  const usedFragments = new Set();
  function markUsed(nodes) {
    for (const node of nodes) {
      if (node.kind === "FragmentSpread" && !usedFragments.has(node.name)) {
        usedFragments.add(node.name);
        const fragment = fragments.get(node.name);
        if (fragment) markUsed(fragment.selection);
      }
      if (node.selection) markUsed(node.selection);
    }
  }
  for (const node of operations) markUsed(node.selection);
  const cyclic = new Set();
  function cycle(node, trail) {
    for (const selection of node.selection || []) {
      if (selection.kind === "FragmentSpread") {
        if (trail.includes(selection.name)) trail.slice(trail.indexOf(selection.name)).forEach(name => cyclic.add(name));
        else if (fragments.has(selection.name)) cycle(fragments.get(selection.name), [...trail, selection.name]);
      } else cycle(selection, trail);
    }
  }
  for (const [name, node] of fragments) cycle(node, [name]);
  for (const name of cyclic) validation("FragmentCycle", [name], "Fragment cycles not allowed", [fragments.get(name)]);
  function expand(nodes, type, path, trail = []) {
    const out = [];
    for (const node of nodes) {
      for (const directive of node.directives) {
        if (!["include", "skip"].includes(directive.name)) validation("UnknownDirective", [...path, node.name].filter(Boolean), `Unknown directive '${directive.name}'`, [directive]);
      }
      if (node.kind === "Field") { out.push(node); continue; }
      if (!included(node, variables)) continue;
      let fragment = node;
      if (node.kind === "FragmentSpread") {
        fragment = fragments.get(node.name);
        if (!fragment) { validation("UndefinedFragment", path, `Undefined fragment '${node.name}'`, [node]); continue; }
        usedFragments.add(node.name);
        if (trail.includes(node.name) || cyclic.has(node.name)) continue;
        if (fragment.type !== type) { validation("InvalidFragmentType", path, `Fragment '${node.name}' cannot be spread here as objects of type '${type}' can never be of type '${fragment.type}'`, [node]); continue; }
      }
      if (fragment.type && fragment.type !== type) continue;
      out.push(...expand(fragment.selection, type, path, [...trail, node.name]));
    }
    const seen = new Map();
    for (const node of out) {
      const key = keyFor(node), prior = seen.get(key);
      if (prior && prior.name !== node.name) validation("FieldsConflict", path, `'${key}' : '${prior.name}' and '${node.name}' are different fields`, [prior, node]);
      seen.set(key, node);
    }
    return out;
  }
  const loaded = new Map();
  async function load(name) {
    if (!loaded.has(name)) {
      const schema = await describe(name);
      const fields = schema.fields.filter(field => field.accessible !== false && !["address", "location", "base64", "blob", "complexvalue"].includes(fieldType(field))).map(field => field.name);
      const query = "SELECT " + fields.join(",") + " FROM " + name;
      const result = await jsonRequest(rest + "/query?q=" + encodeURIComponent(query));
      const records = [...result.records];
      let next = result.nextRecordsUrl;
      while (next) { const page = await jsonRequest(next); records.push(...page.records); next = page.nextRecordsUrl; }
      for (const record of records) {
        for (const field of schema.fields) {
          // Salesforce stores an empty text value as null; keep false and zero.
          if (record[field.name] === "" && ["string", "picklist", "multipicklist"].includes(fieldType(field))) record[field.name] = null;
          // The local SOQL endpoint retains decimal precision as text. UI API
          // numeric scalars are numbers for filtering, ordering and projection.
          const value = record[field.name];
          if (numericTypes.has(fieldType(field)) && typeof value === "string" && value !== "" && Number.isFinite(Number(value))) record[field.name] = Number(value);
        }
      }
      loaded.set(name, { schema, records });
    }
    return loaded.get(name);
  }
  function fetching(node, path, detail) {
    error(`Exception while fetching data (/${path.join("/")}) : ${detail}`, [node], "DataFetchingException", path);
  }
  // Validate the selection AST against schema, without loading or resolving a
  // record. Empty connections and null references have the same field contract
  // as populated connections and references (API 59/67 review controls).
  async function validateSelections(nodes, type, path) {
    for (const node of expand(nodes, type, path)) {
      if (!included(node, variables) || node.name === "__typename") continue;
      const field = node.name, current = [...path, field];
      let childType;
      if (type === "Query" && field === "uiapi") childType = "UIAPI";
      else if (type === "UIAPI" && field === "query") childType = "RecordQuery";
      else if (type === "RecordQuery") {
        try { await describe(field); childType = field + "Connection"; }
        catch (failure) {
          if (failure.status !== 404) throw failure;
        }
      } else if (type.endsWith("Connection")) {
        if (field === "edges") childType = type.slice(0, -10) + "Edge";
        else if (field === "pageInfo") childType = "PageInfo";
        else if (field === "totalCount") continue;
      } else if (type.endsWith("Edge")) {
        if (field === "node") childType = type.slice(0, -4);
        else if (field === "cursor") continue;
      } else if (type === "PageInfo" || type.endsWith("Value")) {
        // These scalar wrapper projections are resolved by their existing path.
        continue;
      } else if (type !== "Query" && type !== "UIAPI") {
        const schema = await describe(type);
        const metadata = schema.fields.find(item => item.name === field);
        const relation = schema.fields.find(item => item.relationshipName === field && fieldType(item) === "reference");
        if (field === "Id") {
          if (node.selection.length) validation("SubselectionNotAllowed", current, "Subselection not allowed on leaf type 'ID!' of field 'Id'", [node]);
          continue;
        }
        if (relation) childType = relation.referenceTo[0];
        else if (metadata) {
          const kind = fieldType(metadata);
          const scalar = scalarTypes[kind] || (kind === "decimal" ? "Double" : "String");
          if (!node.selection.length) validation("SubselectionRequired", current, `Subselection required for type '${scalar}Value' of field '${field}'`, [node]);
          childType = scalar + "Value";
        }
      }
      if (childType) await validateSelections(node.selection, childType, current);
      else validation("FieldUndefined", current, `Field '${field}' in type '${type}' is undefined`, [node]);
    }
  }
  async function project(nodes, type, context, path) {
    const result = Object.create(null);
    for (const node of expand(nodes, type, path)) {
      if (!included(node, variables)) continue;
      const key = keyFor(node), field = node.name, current = [...path, field];
      let value;
      if (field === "__typename") value = type;
      else if (type === "Query" && field === "uiapi") value = await project(node.selection, "UIAPI", null, current);
      else if (type === "UIAPI" && field === "query") value = await project(node.selection, "RecordQuery", null, current);
      else if (type === "RecordQuery") {
        let data;
        try { data = await load(field); }
        catch (failure) {
          if (failure.status !== 404 || !failure.url?.endsWith("/describe")) throw failure;
          validation("FieldUndefined", current, `Field '${field}' in type '${type}' is undefined`, [node]); continue;
        }
        if (hasSelection(node.selection, child => child.kind === "InlineFragment" && !child.type)) {
          fetching(node, current, 'Cannot invoke "graphql.language.TypeName.getName()" because the return value of "graphql.language.InlineFragment.getTypeCondition()" is null');
          continue;
        }
        for (const arg of node.args) {
          if (!["where", "orderBy", "first", "after", "upperBound", "scope"].includes(arg.name)) validation("UnknownArgument", current, `Unknown field argument '${arg.name}'`, [arg]);
        }
        const args = argumentsFor(node, variables);
        const whereArg = node.args.find(arg => arg.name === "where");
        function validateFilter(filter, prefix = "where") {
          for (const [name, operators] of Object.entries(filter || {})) {
            if (["and", "or"].includes(name)) {
              if (name === "or" && operators === null) {
                fetching(node, current, 'Cannot invoke "java.util.List.iterator()" because "conditionList" is null');
                continue;
              }
              operators.forEach((item, index) => validateFilter(item, `${prefix}.${name}[${index}]`));
              continue;
            }
            if (name === "not") {
              if (operators === null) fetching(node, current, 'Cannot invoke "java.util.Map.entrySet()" because "conditions" is null');
              else validateFilter(operators, prefix + ".not");
              continue;
            }
            const metadata = data.schema.fields.find(f => f.name === name);
            const scalar = scalarTypes[fieldType(metadata)] || "String";
            for (const [op, operand] of Object.entries(operators || {})) {
              // Comparison predicates have no local execution contract. Do not
              // coerce other scalar types with JavaScript comparison operators.
              if (["gt", "gte", "lt", "lte"].includes(op)) {
                throw new Error("Local UI GraphQL comparison predicates are not supported");
              } else if (!["eq", "ne", "in", "nin"].includes(op) && !(op === "like" && scalar === "String")) {
                validation("WrongType", current, `argument '${prefix}.${name}' with value '${javaValue(whereArg.value)}' contains a field not in '${scalar}Operators': '${op}'`, [whereArg]);
              } else if (op === "nin" && operand === null) {
                fetching(node, current, "The value can't be empty for nin predicate");
              } else if (op === "like" && (operand === null || operand === "")) {
                fetching(node, current, "The value can't be null or an empty string for LIKE predicate");
              }
            }
          }
        }
        if (whereArg) validateFilter(resolvedValue(whereArg.value, variables));
        if (errors.length) continue;
        const filtered = data.records.filter(record => matches(record, args.where));
        const order = Object.entries(args.orderBy || {});
        filtered.sort((left, right) => {
          for (const [name, config] of order) {
            const a = left[name], b = right[name];
            const compared = a === b ? 0 : a == null ? -1 : b == null ? 1 : a < b ? -1 : 1;
            if (compared) return config.order === "DESC" ? -compared : compared;
          }
          return String(left.Id).localeCompare(String(right.Id));
        });
        const first = args.first == null ? 10 : args.first;
        if (first < 0) { fetching(node, current, "You have paginated maximum allowed number of records without using upperBound, to go further please provide upperBound"); continue; }
        let offset = 0;
        if (args.after != null) {
          let decoded = "";
          try { decoded = atob(String(args.after)); } catch { /* Validation below. */ }
          if (!/^v1:\d+$/.test(decoded)) { fetching(node, current, "Could not parse the cursor."); continue; }
          offset = Number(decoded.slice(3)) + 1;
        }
        // The captured zero-page boundary returns the first record while its
        // end cursor precedes the page. Positive sizes use their actual end.
        const rows = filtered.slice(offset, offset + Math.min(Math.max(first, 1), 2000));
        const cursor = index => btoa("v1:" + index);
        const connection = {
          edges: rows.map((record, index) => ({ cursor: cursor(offset + index), node: record })),
          pageInfo: { hasNextPage: offset + rows.length < filtered.length, hasPreviousPage: offset > 0, startCursor: rows.length ? cursor(offset) : null, endCursor: rows.length ? cursor(offset + (first === 0 ? 0 : rows.length) - 1) : null },
          totalCount: filtered.length,
          metadata: data,
        };
        const selection = expand(node.selection, field + "Connection", current);
        if (!selection.some(child => child.name === "edges" && included(child, variables))) {
          // A page-info-only selection does not materialize a record page.
          connection.pageInfo = { hasNextPage: false, hasPreviousPage: false, startCursor: null, endCursor: null };
        }
        // Omitted first exposes the native connection reference; an explicit
        // nullable first still enters the ordinary default-size projection.
        const projected = await project(node.selection, field + "Connection", connection, current);
        value = node.args.some(arg => arg.name === "first") ? projected : connectionReference(current, args);
      } else if (type.endsWith("Connection")) {
        const object = type.slice(0, -10);
        if (field === "edges") value = await Promise.all(context.edges.map(edge => project(node.selection, object + "Edge", { ...edge, metadata: context.metadata }, current)));
        else if (field === "pageInfo") value = await project(node.selection, "PageInfo", context.pageInfo, current);
        else if (field === "totalCount") value = context.totalCount;
        else { validation("FieldUndefined", current, `Field '${field}' in type '${type}' is undefined`, [node]); continue; }
      } else if (type.endsWith("Edge")) {
        if (field === "cursor") value = context.cursor;
        else if (field === "node") value = await project(node.selection, type.slice(0, -4), { record: context.node, metadata: context.metadata }, current);
        else { validation("FieldUndefined", current, `Field '${field}' in type '${type}' is undefined`, [node]); continue; }
      } else if (type === "PageInfo" || type.endsWith("Value")) value = context[field] ?? null;
      else if (context?.record) {
        const metadata = context.metadata.schema.fields.find(f => f.name === field);
        const relation = context.metadata.schema.fields.find(f => f.relationshipName === field && fieldType(f) === "reference");
        if (field === "Id") {
          if (node.selection.length) { validation("SubselectionNotAllowed", current, `Subselection not allowed on leaf type 'ID!' of field 'Id'`, [node]); continue; }
          value = context.record.Id;
        } else if (relation) {
          const parent = await load(relation.referenceTo[0]);
          const record = parent.records.find(row => row.Id === context.record[relation.name]);
          value = record ? await project(node.selection, relation.referenceTo[0], { record, metadata: parent }, current) : null;
        } else if (metadata) {
          const kind = fieldType(metadata);
          const scalar = scalarTypes[kind] || (kind === "decimal" ? "Double" : "String");
          if (!node.selection.length) { validation("SubselectionRequired", current, `Subselection required for type '${scalar}Value' of field '${field}'`, [node]); continue; }
          let raw = context.record[field] ?? null;
          if (kind === "datetime" && raw != null) raw = new Date(raw).toISOString();
          const wantsValue = expand(node.selection, scalar + "Value", current).some(child => child.name === "value" && included(child, variables));
          let displayValue = null;
          if (raw != null && kind === "date") {
            const [year, month, day] = String(raw).split("-").map(Number); displayValue = `${month}/${day}/${year}`;
          }
          if (raw != null && kind === "datetime") displayValue = new Intl.DateTimeFormat("en-US", { timeZone: "America/Los_Angeles", year: "numeric", month: "numeric", day: "numeric", hour: "numeric", minute: "2-digit" }).format(new Date(raw));
          // Native scalar-only display projections are nullable; requesting value
          // retains the wrapper so its raw null can be observed exactly.
          const hasWrapper = wantsValue || ["date", "datetime"].includes(kind) || (scalar === "String" && raw !== null);
          value = hasWrapper ? await project(node.selection, scalar + "Value", { value: raw, displayValue }, current) : null;
        } else { validation("FieldUndefined", current, `Field '${field}' in type '${type}' is undefined`, [node]); continue; }
      } else { validation("FieldUndefined", current, `Field '${field}' in type '${type}' is undefined`, [node]); continue; }
      result[key] = key in result ? merged(result[key], value) : value;
    }
    return result;
  }
  // Reject unknown fragment types before resolving records. A spread to a
  // known but incompatible type is handled at its selection location.
  const builtInTypes = new Set(["Query", "UIAPI", "RecordQuery", "PageInfo", ...Object.values(scalarTypes).map(type => type + "Value")]);
  for (const [name, node] of fragments) {
    if (!builtInTypes.has(node.type)) {
      const object = node.type.replace(/(?:Connection|Edge)$/, "");
      try { await describe(object); }
      catch (failure) {
        if (failure.status !== 404) throw failure;
        validation("UnknownType", [name], `Unknown type '${node.type}'`, []);
      }
    }
  }
  if (errors.length) return { data: undefined, errors: [{ error: errors, errorType: "adapterError" }] };
  await validateSelections(operation.selection, "Query", []);
  if (errors.length) return { data: undefined, errors: [{ error: errors, errorType: "adapterError" }] };
  const data = await project(operation.selection, "Query", null, []);
  for (const [name, node] of fragments) {
    if (!usedFragments.has(name) && !cyclic.has(name)) validation("UnusedFragment", [], `Unused fragment '${name}'`, [node]);
  }
  return errors.length ? { data: undefined, errors: [{ error: errors, errorType: "adapterError" }] } : { data, errors: undefined };
}
