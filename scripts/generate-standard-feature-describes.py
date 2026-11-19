#!/usr/bin/env python3
"""Pack the owned feature-profile and messaging campaign describe observations."""
import argparse
import csv
import io
import gzip
import json
import re
import xml.etree.ElementTree as ET
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
CAPTURE = ROOT / "internal/apextest/testdata/conformance/standard_describe_profiles/b5b9_feature_objects"
OUTPUT = ROOT / "internal/storage/standard_feature_describes.json.gz"
PROFILES = ("npc-min", "npc-references", "docgen-min", "publicsector", "template")
APIS = tuple(f"{n}.0" for n in range(62, 68))

# The messaging_campaign_presence packet captures MessagingSession: per-API
# source and REST membership for the campaign reference, dynamic-query
# membership, its describe facets, and custom-field compile routes. The
# object's full describe is not part of the packet, so it never enters the
# objects section that overrides the standard catalog entry.
MESSAGING = ROOT / "internal/apextest/testdata/conformance/messaging_campaign_presence"
MESSAGING_OBJECT = "MessagingSession"
MESSAGING_CLASS_PREFIX = {
    "custom-project": "MsgCustom",
    "custom-adjacent-project": "MsgAdjacent",
    "custom-versions-project": "MsgAdjacent",
    "routes-project": "MsgRoute",
}


def messaging_rows(name):
    with (MESSAGING / name).open(newline="") as source:
        return list(csv.DictReader(source, delimiter="\t"))


def messaging_fixture_field(project, case, api):
    path = MESSAGING / project / "force-app/main/default/classes" / f"{MESSAGING_CLASS_PREFIX[project]}{case}{api[:-2]}.cls"
    if not path.exists():
        raise ValueError(f"missing messaging fixture class: {path}")
    references = re.findall(r"\.([A-Za-z][A-Za-z0-9_]*)\s*=", path.read_text())
    if len(references) != 1:
        raise ValueError(f"missing or repeated messaging field reference: {path}")
    return references[0]


def messaging_declared_fields(project):
    declared = set()
    fields = MESSAGING / project / "force-app/main/default/objects/MessagingSession/fields"
    if not fields.is_dir():
        return declared
    for path in sorted(fields.glob("*.field-meta.xml")):
        root = ET.parse(path).getroot()
        fullName = next((element.text for element in root.iter() if element.tag.split("}")[-1] == "fullName"), None)
        if fullName is None:
            raise ValueError(f"missing messaging fixture field name: {path}")
        declared.add(fullName)
    return declared


def messaging_route_pairs(name):
    rows = {}
    for row in messaging_rows(name):
        if row["route"] not in ("anonymous", "istest") or row["present"] not in ("true", "false"):
            raise ValueError(f"invalid messaging membership row: {row}")
        pair = rows.setdefault(row["api"], {})
        if row["route"] in pair:
            raise ValueError(f"repeated messaging membership row: {row}")
        pair[row["route"]] = row["present"] == "true"
    out = {}
    for api, pair in rows.items():
        if set(pair) != {"anonymous", "istest"} or pair["anonymous"] != pair["istest"]:
            raise ValueError(f"messaging membership routes differ: {name} {api}")
        out[api] = pair["anonymous"]
    return out


def normalize_captured_facets(facet):
    facet = dict(facet)
    facet["type"] = {"integer": "int"}.get(facet["type"].lower(), facet["type"].lower())
    return facet


def pack_messaging_capture():
    """Pack the messaging campaign presence rows into bundle sections."""
    membership = messaging_route_pairs("versions.tsv")
    api68 = messaging_route_pairs("api68.tsv")
    for api in api68:
        if api in membership:
            raise ValueError(f"repeated messaging membership capture: {api}")
    membership.update(api68)
    for row in messaging_rows("rest-versions.tsv"):
        if row["result"] not in ("absent", "present"):
            raise ValueError(f"invalid messaging REST row: {row}")
        present = row["result"] == "present"
        if row["api"] in membership and membership[row["api"]] != present:
            raise ValueError(f"messaging REST membership disagrees: {row['api']}")
        membership[row["api"]] = present

    custom = {}

    def record_custom(field, route, accepted):
        if not field.endswith("__c"):
            return  # The standard-shaped control probe is not a custom field.
        routes = custom.setdefault(field, {})
        if route in routes and routes[route] != accepted:
            raise ValueError(f"messaging custom routes differ: {field} {route}")
        routes[route] = accepted

    def record_custom_named(project, case, api, result):
        if result not in ("accept", "reject"):
            raise ValueError(f"invalid messaging named row: {project} {case} {api}")
        field = messaging_fixture_field(project, case, api)
        accepted = result == "accept"
        if accepted != (field in messaging_declared_fields(project)):
            raise ValueError(f"messaging named route disagrees with deployment: {project} {case} {api}")
        record_custom(field, "named", accepted)

    named_cases = {}
    for row in messaging_rows("custom-fields.tsv"):
        named_cases[row["case"]] = messaging_fixture_field("custom-project", row["case"], row["api"])
        record_custom_named("custom-project", row["case"], row["api"], row["result"])
    for filename, project in (("custom-adjacent.tsv", "custom-adjacent-project"),
                              ("custom-versions.tsv", "custom-versions-project")):
        for row in messaging_rows(filename):
            if row["route"] not in ("named", "anonymous") or row["result"] not in ("accept", "reject"):
                raise ValueError(f"invalid messaging custom route: {row}")
            if row["route"] == "named":
                record_custom_named(project, row["case"], row["api"], row["result"])
            else:
                record_custom(messaging_fixture_field(project, row["case"], row["api"]),
                              "anonymous", row["result"] == "accept")
    for row in messaging_rows("custom-anonymous.tsv"):
        marker = "Variable does not exist: "
        at = row["problem"].find(marker)
        if at < 0:
            raise ValueError(f"unrecognized messaging anonymous probe: {row}")
        field = row["problem"][at + len(marker):].strip()
        if not field.endswith("__c"):
            continue  # The standard-shaped control probe is not a custom field.
        if field != named_cases.get(row["case"], field):
            raise ValueError(f"messaging anonymous probe disagrees with named capture: {row}")
        if row["result"] not in ("accept", "reject"):
            raise ValueError(f"invalid messaging anonymous row: {row}")
        record_custom(field, "anonymous", row["result"] == "accept")

    query = {}
    for filename in ("routes.tsv", "query-versions.tsv"):
        for row in messaging_rows(filename):
            api = row["api"]
            if api not in membership:
                raise ValueError(f"uncaptured messaging route API: {api}")
            if row["route"] == "named-valid":
                if (row["result"] == "accept") != membership[api]:
                    raise ValueError(f"messaging named-valid route disagrees: {api}")
            elif row["route"] == "named-invalid":
                record_custom_named("routes-project", "Invalid", api, row["result"])
            elif row["route"] == "anonymous":
                if (row["result"] == "success") != membership[api]:
                    raise ValueError(f"messaging anonymous route disagrees: {api}")
            elif row["route"] == "dynamic-query":
                allowed = row["result"] == "success"
                if api in query and query[api] != allowed:
                    raise ValueError(f"messaging dynamic-query rows differ: {api}")
                query[api] = allowed
            elif row["route"] == "static-query":
                if (row["result"] == "accept") != membership[api]:
                    raise ValueError(f"messaging static-query route disagrees: {api}")
            elif row["route"] == "rest-describe":
                if (row["result"] == "present") != membership[api]:
                    raise ValueError(f"messaging REST route disagrees: {api}")
            else:
                raise ValueError(f"unrecognized messaging route: {row}")

    facets = {}
    for row in messaging_rows("facets.tsv"):
        if row["api"] in facets:
            raise ValueError(f"repeated messaging facets: {row['api']}")
        facets[row["api"]] = normalize_captured_facets(json.loads(row["facets"]))
    captured = []
    for api in sorted(facets):
        facet = facets[api]
        duplicate = False
        for existing in captured:
            if existing["name"] != facet["name"]:
                continue
            if existing != facet:
                raise ValueError(f"messaging facets differ across captured APIs: {facet['name']}")
            duplicate = True
        if not duplicate:
            captured.append(facet)
    for row in messaging_rows("facets-istest.tsv"):
        if facets.get(row["api"]) != normalize_captured_facets(json.loads(row["facets"])):
            raise ValueError(f"messaging istest facets differ: {row['api']}")

    for field, routes in custom.items():
        if set(routes) != {"anonymous", "named"}:
            raise ValueError(f"incomplete messaging custom routes: {field}")
    field = messaging_fixture_field("routes-project", "Valid", "62.0")
    if {facet["name"] for facet in captured} != {field}:
        raise ValueError(f"messaging facets disagree with the captured field: {field}")
    if not set(APIS) <= set(membership):
        raise ValueError(f"incomplete messaging membership matrix: {sorted(set(APIS) - set(membership))}")
    return ({MESSAGING_OBJECT: {field: membership}},
            {MESSAGING_OBJECT: {field: query}},
            {MESSAGING_OBJECT: captured},
            {MESSAGING_OBJECT: custom})


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
    with (CAPTURE / "org.tsv").open(newline="") as source:
        for row in csv.DictReader(source, delimiter="\t"):
            if row["profile"] not in ("gift-bisect", "doc-bisect"):
                continue
            if row["route"] not in ("anonymous", "istest"):
                continue
            obj, field = row["case"].split(".", 1)
            key = (obj, field, row["api"])
            pair = routes.setdefault(key, {})
            if row["route"] in pair or row["result"] not in ("true", "false"):
                raise ValueError(f"duplicate or invalid native field row: {key}")
            pair[row["route"]] = row["result"] == "true"
    for (obj, field, api), pair in routes.items():
        if set(pair) != {"anonymous", "istest"} or pair["anonymous"] != pair["istest"]:
            raise ValueError(f"native routes differ: {obj}.{field} {api}")
        versions.setdefault(obj, {}).setdefault(field, {})[api] = pair["anonymous"]
    for obj, fields in versions.items():
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
    messaging_versions, query_versions, describe_facets, custom_field_routes = pack_messaging_capture()
    for obj, fields in messaging_versions.items():
        if obj in versions:
            raise ValueError(f"repeated versioned native object: {obj}")
        for field, apis in fields.items():
            if field not in {facet["name"] for facet in describe_facets[obj]}:
                raise ValueError(f"messaging version field without captured facets: {obj}.{field}")
    versions.update(messaging_versions)
    payload = json.dumps({"readOnly": readonly, "objects": objects, "fieldVersions": versions,
                          "namedOnly": named_only, "namedQueryVersions": named_queries,
                          "queryVersions": query_versions, "describeFacets": describe_facets,
                          "customFieldRoutes": custom_field_routes},
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
        print(f"wrote {len(objects)} captured objects, {sum(map(len, versions.values()))} versioned fields and {sum(map(len, custom_field_routes.values()))} custom-field routes")


if __name__ == "__main__":
    main()
