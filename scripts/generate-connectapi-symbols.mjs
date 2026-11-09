#!/usr/bin/env node
// Generates the captured ConnectApi shape overlay from testdata/generated/connectapi_shape.json.
//
// The data file is public API shape only. Its sources, recorded per row:
//   * the Tooling API completions symbol table (type names, constructors, methods,
//     property names) of a fresh scratch org;
//   * compile probes deployed check-only at API 62.0 and 67.0: `new ConnectApi.X()`
//     for constructibility and `Blob b = o.prop;` for each property, whose first compile
//     problem names the property type or its version boundary;
//   * runtime probes in the same org: String.valueOf(new ConnectApi.X()) for inherited
//     properties, X.values() for enum constants and Type.isAssignableFrom for superclasses.
// Only rows that compile natively at API 62.0 or 67.0 become symbols. Members that the
// org reports as added after 67.0 or removed before 62.0 become version ranges, so the
// semantic checker rejects them the way the org does.
//
// Output: internal/typesys/connectapi_shape_generated.tsv.gz, a gzip (mtime 0, so the
// bytes are reproducible) of one tab-separated record stream that
// internal/typesys/connectapi_symbols.go embeds and decodes on first use. Keeping the
// 6.7k types out of Go literals keeps them out of package init, the data segment and
// every GC root scan; a process that never names ConnectApi never decodes them.
//
// Record stream (one record per line, fields separated by tabs, "-" for no flags):
//   glade-connectapi-shape<TAB>1                    header: format name and version
//   t <Name> <flags>                                 type; flags: e enum, v virtual, r replace constructors
//   c [<param name> <param type>]...                 constructor of the preceding type
//   m <name> <return type> <flags> [<pn> <pt>]...    method of the preceding type; flags: s static, v virtual
//   p <name> <type> <flags>                          property of the preceding type; flags: s static, f force
//   s <lower-case name> <superclass>                 captured superclass
//   r <lower-case owner.member> <since> <until>      member version range (0 for no bound)
//   x <apex:... surface id> <since> <until>          exact static overload range
//   o <lower-case name>                              output (representation) class
//   j <lower-case name> <flags>                      JSON profile of an output class; flags: n omit nulls,
//                                                    w whole value follows (jw), k explicit keys follow
//                                                    (jk), d defaults follow (jd), x failure follows (jx),
//                                                    "-" for none
//   jx <lower-case name> <phase> <exception> <msg>   Type.forName answers null (forName), newInstance throws
//                                                    (newInstance; {T} is the type name) or JSON.serialize
//                                                    throws (serialize)
//   jw <lower-case name> <json literal>              the whole value JSON.serialize writes (null, [])
//   jk <lower-case name> <key>...                    serialized keys in order (JSON.serialize)
//   jd <lower-case name> <key> <json literal> [c]    value serialized for an unassigned property; c: an
//                                                    assignment does not change it
//
// Usage: node scripts/generate-connectapi-symbols.mjs [data.json] [archive-out.tsv.gz]
import fs from "node:fs";
import path from "node:path";
import zlib from "node:zlib";

const repoRoot = path.resolve(new URL("..", import.meta.url).pathname);
const dataFile = path.resolve(process.argv[2] || path.join(repoRoot, "testdata", "generated", "connectapi_shape.json"));
const archiveOut = path.resolve(process.argv[3] || path.join(repoRoot, "internal", "typesys", "connectapi_shape_generated.tsv.gz"));

const data = JSON.parse(fs.readFileSync(dataFile, "utf8"));
const ns = "ConnectApi";

function normalizeType(raw) {
  let out = String(raw || "").trim();
  if (!out) return "Object";
  out = out.replace(/\s+/g, "");
  out = out.replace(/\bSystem\.([A-Za-z_][A-Za-z0-9_]*)\b/g, "$1");
  return out;
}

function range(since, until) {
  const out = {};
  if (since) out.since = since;
  if (until) out.until = until;
  return out;
}

// Short keys keep the decoded document small; internal/typesys/connectapi_symbols.go
// names each one. Omitted keys decode to the Go zero value.
function param(name, type) {
  return { n: name, t: normalizeType(type) };
}

const sharedInstanceMethods = new Set(["clone", "equals", "hashCode", "toString", "getBuildVersion"]);
const types = [];
const superClasses = {};
const memberRanges = new Map();
const outputs = [];
const jsonProfiles = (data.jsonProfiles || []).map((profile) => [`${ns}.${profile.name}`.toLowerCase(), profile]);
let typeCount = 0;
let enumCount = 0;
let propertyCount = 0;

for (const type of data.types) {
  const full = `${ns}.${type.name}`;
  const lowerFull = full.toLowerCase();
  typeCount++;
  const out = { n: full };
  if (type.kind === "enum") {
    enumCount++;
    out.k = "enum";
    if (type.new) {
      out.m = [
        { n: "ordinal", r: "Integer" },
        { n: "name", r: "String" },
        { n: "valueOf", r: full, p: [param("str", "String")], s: true },
        { n: "values", r: `List<${full}>`, s: true },
      ];
    }
    const values = type.values || [];
    if (values.length) {
      out.p = values.map((value) => ({ n: value, t: full, s: true }));
    }
    types.push(out);
    continue;
  }
  if (type.output) {
    outputs.push(lowerFull);
  }
  if (type.super || type.superQualified) {
    superClasses[lowerFull] = type.superQualified || `${ns}.${type.super}`;
  }
  if (type.virtual) {
    out.v = true;
  }
  if (type.ctor === "none") {
    out.rc = true;
  } else if (type.new && type.ctors && type.ctors.length) {
    out.c = type.ctors.map((ctor) => ctor.map((t, i) => param(`arg${i}`, t)));
  }
  if (type.ctorSince) {
    memberRanges.set(`${lowerFull}.${type.name.toLowerCase()}`, range(type.ctorSince, 0));
  }
  // Static service methods are emitted when their @IsTest probe row compiled
  // natively (staticMethods); setTest counterparts come from the same signatures.
  const instanceMethods = type.new ? (type.methods || []).filter((m) => !m.static && sharedInstanceMethods.has(m.name)) : [];
  const virtualMethods = (type.virtualMethods || []).map((m) => ({ ...m, virtual: true }));
  const emitted = [...instanceMethods, ...virtualMethods, ...(type.staticMethods || [])];
  if (emitted.length) {
    out.m = emitted.map((method) => {
      const entry = { n: method.name, r: normalizeType(method.returnType || "void") };
      const params = (method.params || []).map((p, i) => param(p.name || `arg${i}`, p.type));
      if (params.length) entry.p = params;
      if (method.static) entry.s = true;
      if (method.virtual) entry.v = true;
      return entry;
    });
  }
  const typed = (type.props || []).filter((p) => p.type);
  if (typed.length) {
    propertyCount += typed.length;
    out.p = typed.map((prop) => ({ n: prop.name, t: normalizeType(prop.type), f: true }));
  }
  for (const prop of type.props || []) {
    if (prop.since || prop.until) {
      memberRanges.set(`${lowerFull}.${prop.name.toLowerCase()}`, range(prop.since, prop.until));
    }
  }
  types.push(out);
}

// Static method boundaries: exact signature keys, plus a name key when every
// overload of that name shares one range.
const exactRanges = new Map();
for (const r of data.methodRanges || []) {
  const owner = `${ns}.${r.type}`.toLowerCase();
  const params = r.params.map((p) => normalizeType(p).toLowerCase()).join(",");
  const literal = range(r.since, r.until);
  exactRanges.set(`apex:${owner}.${r.name.toLowerCase()}(${params})`, literal);
  if (r.allOverloads) {
    memberRanges.set(`${owner}.${r.name.toLowerCase()}`, literal);
  }
}

function sortedEntries(map) {
  return [...map.entries()].sort((a, b) => (a[0] < b[0] ? -1 : a[0] > b[0] ? 1 : 0));
}

function flags(...pairs) {
  const out = pairs.filter(([on]) => on).map(([, letter]) => letter).join("");
  return out || "-";
}

const records = [["glade-connectapi-shape", "1"]];
for (const type of types) {
  records.push(["t", type.n, flags([type.k === "enum", "e"], [type.v, "v"], [type.rc, "r"])]);
  for (const ctor of type.c || []) {
    records.push(["c", ...ctor.flatMap((p) => [p.n, p.t])]);
  }
  for (const method of type.m || []) {
    records.push(["m", method.n, method.r, flags([method.s, "s"], [method.v, "v"]), ...(method.p || []).flatMap((p) => [p.n, p.t])]);
  }
  for (const prop of type.p || []) {
    records.push(["p", prop.n, prop.t, flags([prop.s, "s"], [prop.f, "f"])]);
  }
}
for (const [name, parent] of Object.entries(superClasses)) {
  records.push(["s", name, parent]);
}
for (const [tag, map] of [["r", memberRanges], ["x", exactRanges]]) {
  for (const [key, r] of sortedEntries(map)) {
    records.push([tag, key, String(r.since || 0), String(r.until || 0)]);
  }
}
for (const name of outputs.sort()) {
  records.push(["o", name]);
}
for (const [name, profile] of jsonProfiles.sort((a, b) => (a[0] < b[0] ? -1 : a[0] > b[0] ? 1 : 0))) {
  const keys = profile.keys || [];
  const defaults = Object.entries(profile.defaults || {});
  records.push(["j", name, flags([profile.omitNulls, "n"], [profile.whole, "w"], [keys.length, "k"], [defaults.length, "d"], [profile.fail, "x"])]);
  if (profile.fail) records.push(["jx", name, ...profile.fail]);
  if (profile.whole) records.push(["jw", name, profile.whole]);
  if (keys.length) records.push(["jk", name, ...keys]);
  const constants = new Set(profile.constants || []);
  for (const [key, literal] of defaults) records.push(["jd", name, key, literal, ...(constants.has(key) ? ["c"] : [])]);
}
for (const record of records) {
  for (const field of record) {
    if (field === "" || /[\t\n\r]/.test(field)) {
      throw new Error(`unencodable field ${JSON.stringify(field)} in ${JSON.stringify(record)}`);
    }
  }
}

const text = Buffer.from(records.map((record) => record.join("\t")).join("\n") + "\n", "utf8");
const archive = zlib.gzipSync(text, { level: 9 });
fs.writeFileSync(archiveOut, archive);
console.log(
  `connectapi: ${typeCount} types, ${enumCount} enums, ${propertyCount} typed properties, ${Object.keys(superClasses).length} superclasses, ${memberRanges.size} member ranges, ${exactRanges.size} exact ranges; ${text.length} bytes text, ${archive.length} bytes gzip`,
);
