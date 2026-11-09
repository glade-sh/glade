#!/usr/bin/env python3
"""Fold the native ConnectApi JSON sweep into testdata/generated/connectapi_shape.json.

Inputs (native rows, see ORG-RUN.txt beside them):
  internal/apextest/testdata/conformance/capi4_output_json/org.tsv
      JSON.serialize(Type.forName(name).newInstance()) for every ConnectApi type
      at class API versions 62.0, 67.0 and 68.0.
  internal/apextest/testdata/conformance/capi4_output_json/props.tsv
      one compile probe per (type, property) where the serialized keys and the
      shape catalog disagreed.

The script edits the shape data (the authoritative input of
scripts/generate-connectapi-symbols.mjs), then that generator rebuilds
internal/typesys/connectapi_shape_generated.tsv.gz. It is idempotent.

Catalog edits
  * a serialized key with no catalog property becomes a property added at 68.0
    (type from the 68.0 compile probe), declared on the highest ancestor whose
    probe compiles;
  * a catalog property whose probe says "removed after version N" gets until N.

Top-level "jsonProfiles" (one row per type whose native serialization differs from
the default: every property that is not removed, ascending by name, null as null):
  omitNulls  the type drops null properties (it serialized no null)
  whole      the type serializes as this JSON text (null, [])
  keys       the explicit key order, when it is not the default
  defaults   non-null values native serializes for unassigned properties
  constants  the defaults that an assignment does not change (cells.tsv)
  fail       [phase, exception, message]: Type.forName answers null (forName),
             newInstance throws ({T} stands for the type name) or JSON.serialize
             throws (serialize)
"""
import json
import re
import sys
from pathlib import Path

root = Path(__file__).resolve().parent.parent
shape_path = root / "testdata/generated/connectapi_shape.json"
conf = root / "internal/apextest/testdata/conformance/capi4_output_json"

raw = shape_path.read_text()
head, _, _ = raw.partition('"types": [\n')
data = json.loads(raw)
types = data["types"]
by_name = {"ConnectApi." + t["name"]: t for t in types}


def chain(name):
    out = []
    while name in by_name:
        out.append(name)
        sup = by_name[name].get("super")
        if not sup:
            break
        name = sup if "." in sup else "ConnectApi." + sup
    return out


def rows(path):
    lines = path.read_text().splitlines()
    cols = lines[0].split("\t")
    return [dict(zip(cols, line.split("\t"))) for line in lines[1:]]


sweep = {r["name"]: r["api67"] for r in rows(conf / "org.tsv")}
probes = {(r["type"], r["prop"]): r for r in rows(conf / "props.tsv")}
cells = {(r["type"], r["prop"]): r["native"] for r in rows(conf / "cells.tsv")}


def parse_probe(message):
    m = re.match(r"Property not added until version (\d+)\.0: ", message)
    if m:
        return ("since", int(m[1]))
    m = re.match(r"Property was removed after version (\d+)\.0: ", message)
    if m:
        return ("until", int(m[1]))
    m = re.match(r"Illegal assignment from (.*) to Blob", message)
    if m:
        return ("ok", m[1])
    return ("none", message)


def ordered_pairs(text):
    """Top-level (key, value) pairs in order, duplicates kept; nested values are plain JSON."""
    top = json.loads(text)
    pairs = json.loads(text, object_pairs_hook=lambda p: p)
    if not isinstance(pairs, list) or not isinstance(top, dict):
        return pairs
    return [(key, top[key]) for key, _ in pairs]


def all_props(name):
    props = {}
    for owner in chain(name):
        for prop in by_name[owner].get("props", []):
            props.setdefault(prop["name"], (owner, prop))
    return props


# 1. removed properties
for (typ, prop), row in probes.items():
    if typ not in by_name:
        continue
    for entry in by_name[typ].get("props", []):
        if entry["name"] != prop:
            continue
        removed = [parse_probe(row[a]) for a in ("api62", "api67", "api68")]
        until = [v for k, v in removed if k == "until"]
        if until and not entry.get("until"):
            entry["until"] = until[-1]

# 2. serialized keys the catalog lacks
added = 0
for name, text in sorted(sweep.items()):
    if name not in by_name or not text.startswith("{"):
        continue
    pairs = ordered_pairs(text)
    if not isinstance(pairs, list):
        continue
    known = all_props(name)
    for key, _ in pairs:
        if key in known:
            continue
        owner = None
        for candidate in reversed(chain(name)):
            row = probes.get((candidate, key))
            if row and parse_probe(row["api68"])[0] == "ok":
                owner = candidate
                break
        if owner is None:
            continue
        row = probes[(owner, key)]
        kind62 = parse_probe(row["api62"])
        entry = {"name": key, "type": parse_probe(row["api68"])[1]}
        if kind62[0] == "since":
            entry["since"] = kind62[1]
        props = by_name[owner].setdefault("props", [])
        props.append(entry)
        props.sort(key=lambda p: p["name"])
        added += 1
        known = all_props(name)


def default_keys(name):
    return sorted(k for k, (_, p) in all_props(name).items() if not p.get("until"))


# 3. JSON profiles, one row per swept type (catalog or not)
catalog = set(by_name)
profiles = []
for name, text in sorted(sweep.items()):
    short = name.removeprefix("ConnectApi.")
    if text == "null" or text == "[]":
        profiles.append({"name": short, "whole": text})
        continue
    if text.startswith("X:"):
        _, exception, message = text.split(":", 2)
        fail = None
        if exception == "System.NullPointerException":
            # Type.forName answered null, so newInstance dereferenced it.
            fail = ["forName", exception, message]
        elif exception == "System.TypeException":
            # "Type is not visible" is the message of a service class the sweep
            # org did not enable; where it is visible the same call reports
            # that the class has no no-arg constructor.
            if message == "Type is not visible: " + name:
                message = name + " does not have a no-arg constructor"
            fail = ["newInstance", exception, message.replace(name, "{T}")]
        elif exception == "System.JSONException" and message.startswith("Cannot invoke "):
            fail = ["serialize", exception, message]
        if fail:
            profiles.append({"name": short, "fail": fail})
        continue
    if not text.startswith("{"):
        continue
    pairs = ordered_pairs(text)
    keys = [k for k, _ in pairs]
    values = dict(pairs)
    profile = {}
    has_null = any(v is None for v in values.values())
    if name in catalog:
        wanted = default_keys(name)
        if not has_null and (not keys or set(wanted) - set(keys)):
            profile["omitNulls"] = True
        if has_null:
            if keys != wanted:
                profile["keys"] = keys
        else:
            # Only non-null keys are observable: keep their order and let the
            # rest follow in the default order.
            rest = [k for k in wanted if k not in keys]
            merged = sorted(keys + rest)
            if keys != [k for k in merged if k in keys] or set(keys) - set(wanted) or len(set(keys)) != len(keys):
                profile["keys"] = keys + rest
    else:
        # Not in the shape catalog: the property set is unknown, so the
        # nulls are dropped unless the sweep saw one.
        if not has_null:
            profile["omitNulls"] = True
        if len(set(keys)) != len(keys):
            profile["keys"] = keys
    defaults = {k: v for k, v in pairs if v is not None}
    if defaults:
        profile["defaults"] = {k: json.dumps(v, separators=(",", ":"), ensure_ascii=False) for k, v in defaults.items()}
        # A default that survives an assignment of another value is a constant;
        # an unprobed string default is a discriminator, so a constant too.
        constants = []
        for k, v in defaults.items():
            probed = cells.get((name, k))
            if probed is not None:
                if probed == json.dumps(v, separators=(",", ":"), ensure_ascii=False):
                    constants.append(k)
            elif isinstance(v, str) and v != "<uuid>":
                constants.append(k)
        if constants:
            profile["constants"] = constants
    if profile:
        profile["name"] = short
        profiles.append(profile)

body = ",\n".join("  " + json.dumps(t, sort_keys=True, separators=(",", ":"), ensure_ascii=False) for t in types)
extra = ",\n".join("  " + json.dumps(p, sort_keys=True, separators=(",", ":"), ensure_ascii=False) for p in profiles)
shape_path.write_text(head + '"types": [\n' + body + "\n ],\n \"jsonProfiles\": [\n" + extra + "\n ]\n}\n")
print(f"added {added} properties, profiled {len(profiles)} types", file=sys.stderr)
