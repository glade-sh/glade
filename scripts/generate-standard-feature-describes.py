#!/usr/bin/env python3
"""Pack the owned feature-profile describe observations for runtime use.

Field versions come from the gift, docgen and auth-flow bisect captures.
Auth-flow rows cover standard catalog objects no profile describe owns, so
version-bisect/authflow-cases.json is their field inventory; their describes
stay in the standard catalog.
"""
import argparse
import csv
import io
import gzip
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
CAPTURE = ROOT / "internal/apextest/testdata/conformance/standard_describe_profiles/b5b9_feature_objects"
OUTPUT = ROOT / "internal/storage/standard_feature_describes.json.gz"
PROFILES = ("npc-min", "npc-references", "docgen-min", "publicsector", "template")
APIS = tuple(f"{n}.0" for n in range(62, 68))
VERSION_BISECTS = ("gift-bisect", "doc-bisect", "authflow-bisect")


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--check", action="store_true", help="verify output without writing")
    args = parser.parse_args()
    objects = {}
    for profile in PROFILES:
        old = json.loads((CAPTURE / f"{profile}-62.0.json").read_text())
        new = json.loads((CAPTURE / f"{profile}-67.0.json").read_text())
        for name, item in new.items():
            if item is None:
                continue
            if name in objects or old.get(name) is None:
                raise ValueError(f"missing or repeated native object {name}")
            for field in item["fields"]:
                field["type"] = {"integer": "int"}.get(field["type"].lower(), field["type"].lower())
            objects[name] = item
    versions = {}
    routes = {}
    version_classes = {}
    with (CAPTURE / "org.tsv").open(newline="") as source:
        for row in csv.DictReader(source, delimiter="\t"):
            if row["profile"] not in VERSION_BISECTS:
                continue
            if row["route"] not in ("anonymous", "istest"):
                continue
            obj, field = row["case"].split(".", 1)
            klass = "authflow" if row["profile"] == "authflow-bisect" else "feature"
            if version_classes.setdefault(obj, klass) != klass:
                raise ValueError(f"version capture class mismatch: {obj}")
            key = (obj, field, row["api"])
            pair = routes.setdefault(key, {})
            if row["route"] in pair or row["result"] not in ("true", "false"):
                raise ValueError(f"duplicate or invalid native field row: {key}")
            pair[row["route"]] = row["result"] == "true"
    for (obj, field, api), pair in routes.items():
        if set(pair) != {"anonymous", "istest"} or pair["anonymous"] != pair["istest"]:
            raise ValueError(f"native routes differ: {obj}.{field} {api}")
        versions.setdefault(obj, {}).setdefault(field, {})[api] = pair["anonymous"]
    authflow_cases = json.loads((CAPTURE / "version-bisect" / "authflow-cases.json").read_text())
    for obj in authflow_cases:
        if obj not in versions:
            raise ValueError(f"auth flow case without native rows: {obj}")
    for obj, fields in versions.items():
        if version_classes[obj] == "authflow":
            if sorted(fields) != sorted(authflow_cases.get(obj, [])):
                raise ValueError(f"auth flow case list mismatch: {obj}")
            for field, apis in fields.items():
                if set(apis) != set(APIS):
                    raise ValueError(f"incomplete native field matrix: {obj}.{field}")
            continue
        if obj not in objects:
            raise ValueError(f"version field without native object: {obj}")
        for field, apis in fields.items():
            if set(apis) != set(APIS) or field not in {f["name"] for f in objects[obj]["fields"]}:
                raise ValueError(f"incomplete native field matrix: {obj}.{field}")
    named62 = json.loads((CAPTURE / "named-shapes/doc-named-facets-62.0.json").read_text())
    named67 = json.loads((CAPTURE / "named-shapes/doc-named-facets-67.0.json").read_text())
    if named62 != named67:
        raise ValueError("named-only field facets differ at captured endpoints")
    named_only = {}
    for field in named67:
        field = dict(field)
        obj = field.pop("object")
        name = field["name"]
        if obj not in objects or name in {f["name"] for f in objects[obj]["fields"]}:
            raise ValueError(f"invalid named-only field {obj}.{name}")
        field["type"] = {"integer": "int"}.get(field["type"].lower(), field["type"].lower())
        objects[obj]["fields"].append(field)
        named_only.setdefault(obj, []).append(name)
    query_routes = {}
    with (CAPTURE / "org.tsv").open(newline="") as source:
        for row in csv.DictReader(source, delimiter="\t"):
            if row["profile"] != "docgen-isolated":
                continue
            if row["route"] == "istest" and row["case"].endswith("-query"):
                obj, field = row["case"][:-len("-query")].split(".", 1)
                query_routes.setdefault((obj, field, row["api"]), {})["endpoint"] = row["result"] == "accepted"
            elif row["route"] in ("named-static-query", "named-dynamic-query"):
                obj, field = row["case"].split(".", 1)
                query_routes.setdefault((obj, field, row["api"]), {})[row["route"]] = row["result"] == "accepted"
    named_queries = {}
    for (obj, field, api), routes in query_routes.items():
        required = {"endpoint"} if api in ("62.0", "67.0") else {"named-static-query", "named-dynamic-query"}
        if set(routes) != required or len(set(routes.values())) != 1:
            raise ValueError(f"incomplete named query controls: {obj}.{field} {api}")
        named_queries.setdefault(obj, {}).setdefault(field, {})[api] = next(iter(routes.values()))
    for obj, fields in named_only.items():
        for field in fields:
            if set(named_queries.get(obj, {}).get(field, {})) != set(APIS):
                raise ValueError(f"incomplete named query matrix: {obj}.{field}")
    # Only promote fields with matching captured static and dynamic rejection
    # at the accessible API boundary; describe facets alone are not an Apex rule.
    readonly = {}
    with (CAPTURE / "org.tsv").open(newline="") as source:
        rows = list(csv.DictReader(source, delimiter="\t"))
    for row in rows:
        if row["profile"] != "gift-readonly" or row["api"] != "67.0" or row["route"] != "anonymous" or not row["result"].endswith(" is not editable"):
            continue
        obj, field, operation = row["case"].split(".")
        if operation != "put":
            raise ValueError(f"unexpected read-only operation: {obj}.{field}")
        for route in ("anonymous", "istest"):
            if not any(r["api"] == "67.0" and r["route"] == route and r["profile"] == "gift-full-write" and r["case"] == obj + "." + field and r["result"] == "rejected" for r in rows):
                raise ValueError(f"missing static read-only control: {obj}.{field} {route}")
            if not any(r["api"] == "67.0" and r["route"] == route and r["profile"] == "gift-readonly" and r["case"] == row["case"] and r["result"].endswith(" is not editable") for r in rows):
                raise ValueError(f"missing dynamic read-only control: {obj}.{field} {route}")
        readonly.setdefault(obj, []).append(field)
    payload = json.dumps({"readOnly": readonly, "objects": objects, "fieldVersions": versions,
                          "namedOnly": named_only, "namedQueryVersions": named_queries},
                         sort_keys=True, separators=(",", ":")).encode()
    target = io.BytesIO()
    with gzip.GzipFile(filename="", mode="wb", fileobj=target, mtime=0) as stream:
        stream.write(payload)
    data = target.getvalue()
    if args.check:
        if not OUTPUT.exists() or OUTPUT.read_bytes() != data:
            raise SystemExit("feature describe sidecar is out of date")
        print("feature describe sidecar is up to date")
    else:
        OUTPUT.write_bytes(data)
        print(f"wrote {len(objects)} captured objects and {sum(map(len, versions.values()))} versioned fields")


if __name__ == "__main__":
    main()
