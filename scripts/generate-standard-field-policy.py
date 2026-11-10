#!/usr/bin/env python3
"""Rebuild the standard field policy from committed, owned native captures.

Only isolated, API-stable field gates are installed. Other profile differences
remain evidence so broader edition/feature support can consume them separately.
"""

import argparse
import csv
import gzip
import hashlib
import io
import json
import os
from pathlib import Path


REPO = Path(__file__).resolve().parents[1]
DEFAULT_CAPTURES = REPO / "internal/apextest/testdata/conformance/standard_describe_profiles"
DEFAULT_OUTPUT = REPO / "internal/storage/standard_field_policy.json.gz"
APIS = ("62.0", "67.0")
PROFILES = tuple(f"P{i}" for i in range(12)) + ("PU",)
CORE_FEATURES = {"P1": "MultiCurrency", "P2": "PersonAccounts", "P3": "StateAndCountryPicklist"}
COMPLETE_CONSTRUCTOR_OBJECTS = ("Contact", "Lead", "PricebookEntry")
PERSON_ADDRESS_FIELDS = (
    "PersonMailingCountryCode", "PersonMailingStateCode",
    "PersonOtherCountryCode", "PersonOtherStateCode",
)


class Inputs:
    def __init__(self, root):
        self.root = Path(root)
        self.sources = {}

    def read(self, path):
        path = self.root / path
        data = path.read_bytes()
        self.sources[os.path.relpath(path, self.root)] = hashlib.sha256(data).hexdigest()
        return data

    def json(self, path):
        data = self.read(path)
        return json.loads(gzip.decompress(data) if str(path).endswith(".gz") else data)

    def tsv(self, path):
        return list(csv.DictReader(io.StringIO(self.read(path).decode()), delimiter="\t"))


def core_gate_membership(rows):
    result = {}
    for profile in CORE_FEATURES:
        by_api = {
            api: {(row["object"], row["field"], row["change"])
                  for row in rows if row["profile"] == profile and row["api"] == api
                  and row["route"] == "effective-membership"}
            for api in APIS
        }
        if not by_api[APIS[0]] or by_api[APIS[0]] != by_api[APIS[1]]:
            raise ValueError(f"{profile} API62 and API67 membership gates differ")
        if any(change != "added" for _, _, change in by_api[APIS[0]]):
            raise ValueError(f"{profile} contains unsupported removed membership gates")
        result[profile] = sorted((obj, field) for obj, field, _ in by_api[APIS[0]])
    return result


def webstore_conditions(rows, license_only_objects):
    for profile in ("P8", "P10"):
        for api in APIS:
            if not any(row["api"] == api and row["profile"] == profile
                       and row["object"] == "WebStore" and row["change"] == "added"
                       for row in rows):
                raise ValueError(f"WebStore addition missing for {profile} at {api}")
    if "WebStore" in license_only_objects:
        raise ValueError("WebStore license-only control is no longer absent")
    return [["OrderManagement"], ["B2BCommerce", "Communities", "CommerceEnabled"]]


def validate_person_address_rows(rows):
    for api in APIS:
        for route in ("anonymous", "named", "test-runner"):
            matches = [row for row in rows if row["api"] == api
                       and row["profile"] == "person-address" and row["route"] == route
                       and row["case"] == "fields"]
            label = f"person-address {api} {route}"
            if len(matches) != 1 or matches[0]["compiled"] != "true":
                raise ValueError(f"missing successful native {label} row")
            observed = json.loads(matches[0]["observation"])
            for field in (*PERSON_ADDRESS_FIELDS, "Name"):
                if observed.get(f"Account.{field}") is not True:
                    raise ValueError(f"native {label} does not expose Account.{field}")


def profile_evidence(field_rows, object_rows):
    evidence = {}
    for profile in PROFILES:
        evidence[profile] = {}
        for api in APIS:
            fields = [row for row in field_rows if row["profile"] == profile
                      and row["api"] == api and row["route"] == "effective-membership"]
            objects = [row for row in object_rows if row["profile"] == profile and row["api"] == api]
            evidence[profile][api] = {
                "baseline_profile": "P4" if profile == "P10" else "P0",
                "fields": {change: sorted([row["object"], row["field"]] for row in fields
                                           if row["change"] == change)
                           for change in ("added", "removed")},
                "objects": {change: sorted(row["object"] for row in objects if row["change"] == change)
                            for change in ("added", "removed")},
            }
    return evidence


def constructor_fields(snapshot, obj, input_fields=()):
    fields = snapshot["describes"][obj]["fields"]
    return sorted({field["name"] for field in fields}
                  | {field["relationshipName"] for field in fields if field.get("relationshipName")}
                  | snapshot["named"].get(obj, set()) | set(input_fields))


def preserved_profile_constructor_fields(snapshots, obj, excluded):
    """Retain the former open admission for known, unqualified profile fields.

    Deferred profile expansion is separate from baseline constructor closure.
    Do not invent new rejections for those captured names or their relationships.
    Captured baseline absences and feature gates keep their existing precedence.
    """
    fields = set()
    for by_api in snapshots.values():
        for value in by_api.values():
            fields.update(value["named"].get(obj, set()) - excluded)
            for field in value["describes"].get(obj, {}).get("fields", []):
                if field["name"] in excluded:
                    continue
                fields.add(field["name"])
                if field.get("relationshipName"):
                    fields.add(field["relationshipName"])
    return fields


def policy_bytes(policy):
    raw = (json.dumps(policy, sort_keys=True, separators=(",", ":")) + "\n").encode()
    # GzipFile writes a stable OS marker as well as a zero timestamp and no name.
    stream = io.BytesIO()
    with gzip.GzipFile(fileobj=stream, mode="wb", filename="", mtime=0) as archive:
        archive.write(raw)
    return stream.getvalue()


def snapshot(inputs, profile, api, objects):
    prefix = f"{profile}/api{api.split('.')[0]}"
    describes = inputs.json(f"{prefix}/granted/describes.json.gz")["describes"]
    describes = {obj: value for obj, value in describes.items() if obj in objects}
    named = {}
    for row in inputs.tsv(f"{prefix}/default/named-fields.tsv"):
        if row["object_present"] == "true" and row["field"]:
            named.setdefault(row["object"], set()).add(row["field"])
    return {"describes": describes, "named": named,
            "global": set(inputs.json(f"{prefix}/granted/global.json")["objects"])}


def membership(value):
    return {(obj, field["name"]) for obj, desc in value["describes"].items() for field in desc["fields"]} | {
        (obj, field) for obj, fields in value["named"].items() for field in fields}


def verify_profile_evidence(snapshots, evidence):
    for profile in PROFILES:
        for api in APIS:
            row = evidence[profile][api]
            baseline = snapshots[row["baseline_profile"]][api]
            candidate = snapshots[profile][api]
            for kind, before, after in (
                ("fields", membership(baseline), membership(candidate)),
                ("objects", baseline["global"], candidate["global"]),
            ):
                for change, observed in (("added", after - before), ("removed", before - after)):
                    expected = {tuple(item) if kind == "fields" else item for item in row[kind][change]}
                    if observed != expected:
                        raise ValueError(f"{profile} {api} {kind} {change} disagree with captured differences")


def api68_profile_evidence(inputs, snapshots, objects):
    """Replay only captured 67-to-68 changes over the retained 67 snapshots."""
    observed = {}
    prefix = "../standard_describe_api68_drift"
    for profile in PROFILES:
        before = snapshots[profile]["67.0"]
        rest = {(obj, field["name"]) for obj, desc in before["describes"].items()
                for field in desc["fields"]}
        named = {(obj, field) for obj, fields in before["named"].items() for field in fields}
        global_names = set(before["global"])
        for suffix in ("granted-rest", "routes", "global"):
            inputs.read(f"{prefix}/{profile}-{suffix}/ORG-RUN.txt")
        for row in inputs.tsv(f"{prefix}/{profile}-granted-rest/org.tsv"):
            if row["object"] not in objects:
                continue
            key = (row["object"], row["field"])
            if row["api67_granted_rest"] not in ("true", "false") or row["api68_granted_rest"] not in ("true", "false"):
                raise ValueError(f"{profile} invalid REST membership for {key}")
            if (key in rest) != (row["api67_granted_rest"] == "true"):
                raise ValueError(f"{profile} 67 REST membership disagrees for {key}")
            (rest.add if row["api68_granted_rest"] == "true" else rest.discard)(key)
        route_rows = inputs.tsv(f"{prefix}/{profile}-routes/org.tsv")
        for row in route_rows:
            if row["api"] != "67.0" or row["context"] != "default" or row["route"] != "named":
                continue
            key = (row["object"], row["field"])
            if row["object"] in objects and ((key in named) != (row["field_present"] == "true")):
                raise ValueError(f"{profile} 67 named membership disagrees for {key}")
        for row in route_rows:
            if row["api"] != "68.0" or row["context"] != "default" or row["route"] != "named":
                continue
            key = (row["object"], row["field"])
            if row["field_present"] not in ("true", "false"):
                raise ValueError(f"{profile} invalid named membership for {key}")
            if row["object"] in objects:
                (named.add if row["field_present"] == "true" else named.discard)(key)
        for row in inputs.tsv(f"{prefix}/{profile}-global/org.tsv"):
            name = row["object"]
            if row["api67_present"] not in ("true", "false") or row["api68_present"] not in ("true", "false"):
                raise ValueError(f"{profile} invalid global membership for {name}")
            if (name in global_names) != (row["api67_present"] == "true"):
                raise ValueError(f"{profile} 67 global membership disagrees for {name}")
            (global_names.add if row["api68_present"] == "true" else global_names.discard)(name)
        observed[profile] = (rest | named, global_names)
    evidence = {}
    for profile in PROFILES:
        baseline = "P4" if profile == "P10" else "P0"
        fields, names = observed[profile]
        base_fields, base_names = observed[baseline]
        evidence[profile] = {
            "baseline_profile": baseline,
            "fields": {"added": sorted(map(list, fields - base_fields)),
                       "removed": sorted(map(list, base_fields - fields))},
            "objects": {"added": sorted(names - base_names),
                        "removed": sorted(base_names - names)},
        }
    return evidence


def require_rows(rows, case, *, profile=None, observation=None, compiled=True, observation_contains=None):
    for api in APIS:
        for route in ("anonymous", "named", "test-runner"):
            matches = [row for row in rows if row["api"] == api and row["route"] == route
                       and (case is None or row.get("case") == case)
                       and (profile is None or row.get("profile") == profile)]
            if len(matches) != 1 or matches[0]["compiled"] != str(compiled).lower():
                raise ValueError(f"missing native {profile or case} {api} {route}")
            if observation is not None and not matches[0]["observation"].startswith(observation):
                raise ValueError(f"unexpected native {profile or case} {api} {route}")
            if observation_contains is not None and observation_contains not in matches[0]["observation"]:
                raise ValueError(f"unexpected native diagnostic for {profile or case} {api} {route}")


def case_source_gate(inputs, snapshots):
    prefix = "../standard_case_source_evidence"
    definition = inputs.json(f"{prefix}/definition.json")
    if (definition.get("features") != ["LiveAgent"]
            or definition.get("settings") != {"liveAgentSettings": {"enableLiveAgent": True}}):
        raise ValueError("Case source profile no longer isolates enabled LiveAgent")
    rows = inputs.tsv(f"{prefix}/org.tsv")
    inputs.read(f"{prefix}/ORG-RUN.txt")
    inputs.json(f"{prefix}/observations.json")
    require_rows(rows, "membership")
    expected = {"SourceId": True, "Subject": True, "OwnedMissing__c": False,
                "HasCommentsUnreadByOwner": False, "IsVisibleInSelfService": False}
    for row in rows:
        if row["case"] == "membership" and json.loads(row["observation"]) != expected:
            raise ValueError("unexpected Case source native membership")
    for api in APIS:
        for case, compiled in (("query", "true"), ("missing-query", "false"),
                               ("boundary-query", "true")):
            matches = [row for row in rows if row["api"] == api
                       and row["route"] == "anonymous" and row["case"] == case]
            if len(matches) != 1 or matches[0]["compiled"] != compiled:
                raise ValueError(f"missing Case source {api} {case} control")
            if case == "missing-query" and "No such column 'OwnedMissing__c'" not in matches[0]["observation"]:
                raise ValueError("Case source rejection is not the missing-field diagnostic")
    # Availability is isolated above. The already captured P11 describes supply
    # rich field facets, without attributing P11's other fields to LiveAgent.
    facets = next(field for field in snapshots["P11"][APIS[0]]["describes"]["Case"]["fields"]
                  if field["name"] == "SourceId")
    return {"object": "Case", "features": ["LiveAgent", "LiveAgentEnabled"], "field": facets}


def qualified_constructor_objects(inputs):
    """Only explicitly qualified FLS-complete objects may acquire a closed list.

    A successful REST describe alone is insufficient: require the direct native
    rejection, accepted boundary and dynamic-put twins on every API/route, and
    bind their source digests to the case table before using the granted surface.
    """
    prefix = "../standard_constructor_complete"
    plan = inputs.json(f"{prefix}/cases.json")
    rows = inputs.tsv(f"{prefix}/org.tsv")
    observations = inputs.json(f"{prefix}/observations.json")
    inputs.read(f"{prefix}/ORG-RUN.txt")
    if (sorted(plan["constructor_complete_objects"]) != sorted(COMPLETE_CONSTRUCTOR_OBJECTS)
            or plan["apis"] != list(APIS)
            or plan["routes"] != ["anonymous", "named", "test-runner"]):
        raise ValueError("constructor qualification requires the explicit captured object/API/route allowlist")
    cases = {(case["object"], case["id"]): case for case in plan["cases"]}
    expected_cases = {(obj, case) for obj in COMPLETE_CONSTRUCTOR_OBJECTS
                      for case in ("missing-constructor", "boundary-constructor", "missing-put")}
    if set(cases) != expected_cases or len(cases) != len(plan["cases"]):
        raise ValueError("constructor qualification case table is incomplete or duplicated")
    expected = {(api, obj, case, route) for api in APIS for obj, case in expected_cases
                for route in plan["routes"]}
    key = lambda row: (row["api"], row["object"], row["case"], row["route"])
    by_key = {key(row): row for row in observations}
    if (len(rows) != len(expected) or len(by_key) != len(expected)
            or len(observations) != len(expected) or set(by_key) != expected
            or {key(row) for row in rows} != expected):
        raise ValueError("constructor qualification native route matrix is incomplete or duplicated")
    for row in rows:
        observed = by_key[key(row)]
        case = cases[row["object"], row["case"]]
        compiled = row["case"] != "missing-constructor"
        if (row["compiled"] != str(compiled).lower() or row["success"] != str(compiled).lower()
                or observed["compiled"] != compiled or observed["success"] != compiled):
            raise ValueError("constructor qualification native compile/runtime result changed")
        source = observed["source"]
        digest = hashlib.sha256(source.encode()).hexdigest()
        if (digest != row["source_sha256"] or digest != observed["source_sha256"]
                or case["body"] not in source
                or row["observation"] != observed["observation"].replace("\n", " ").replace("\r", " ")):
            raise ValueError("constructor qualification source/observation does not match native rows")
        if row["route"] == "anonymous":
            valid_source = source == case["body"] and observed["class_name"] == ""
        else:
            name = observed["class_name"]
            wrapper = (f"public class {name} {{ public static void sample() {{ "
                       if row["route"] == "named" else
                       f"@IsTest private class {name} {{ @IsTest static void sample() {{ ")
            body = case["body"]
            if row["case"] == "missing-put":
                native = by_key[row["api"], row["object"], row["case"], "anonymous"]["observation"]
                body += " System.assertEquals('" + native.replace("'", "\\'") + "', observed);"
            valid_source = bool(name) and source == wrapper + body + " } }"
        if not valid_source:
            raise ValueError("constructor qualification route wrapper does not match captured source")
        if row["case"] == "missing-constructor":
            diagnostic = f"Field does not exist: SyntheticAbsentField on {row['object']}"
            valid = row["observation"].endswith(diagnostic)
        elif row["case"] == "missing-put":
            valid = row["observation"] == f"System.SObjectException|Invalid field SyntheticAbsentField for {row['object']}"
        else:
            valid = row["observation"] == "accepted"
        if not valid:
            raise ValueError("constructor qualification requires the field rejection and accepted boundary")
    return COMPLETE_CONSTRUCTOR_OBJECTS


ENHANCED_ORDER_FEATURE = "EnhancedCommerceOrders"
ENHANCED_ORDER_VARIANTS = ("feature-only", "setting-only", "feature-and-setting", "capitalized-settings")


def enhanced_order_gates(inputs):
    """Gate the Order and OrderItem fields that the enhanced commerce orders
    setting adds.

    The isolated captures vary one thing at a time. The OrderSaveBehaviorBoth
    feature alone adds no field. The orderSettings.enableEnhancedCommerceOrders
    setting adds every gated field, with or without that feature. Only fields
    with identical API62 and API67 membership become gates; the one field that
    appears only at API67 stays out. Rich facets come from the granted describe.
    Picklist value lists are not carried, because their provenance is not the
    reviewed public allowlist.
    """
    prefix = "../standard_enhanced_commerce_orders"
    definitions = {name: inputs.json(f"{prefix}/definitions/{name}.json") for name in ENHANCED_ORDER_VARIANTS}
    settings = {"orderSettings": {"enableOrders": True, "enableEnhancedCommerceOrders": True}}
    if (definitions["feature-only"].get("features") != ["OrderSaveBehaviorBoth"]
            or definitions["feature-only"].get("settings") != {"orderSettings": {"enableOrders": True}}
            or definitions["setting-only"].get("features") is not None
            or definitions["setting-only"].get("settings") != settings
            or definitions["feature-and-setting"].get("features") != ["OrderSaveBehaviorBoth"]
            or definitions["feature-and-setting"].get("settings") != settings
            or definitions["capitalized-settings"].get("features") is not None
            or definitions["capitalized-settings"].get("settings") != {"OrderSettings": settings["orderSettings"]}):
        raise ValueError("enhanced order captures no longer isolate the setting")
    inputs.read(f"{prefix}/ORG-RUN.txt")
    inputs.read(f"{prefix}/ungranted-runtime.tsv")
    members = {}
    for row in inputs.tsv(f"{prefix}/named-fields.tsv"):
        members.setdefault((row["api"], row["variant"]), set()).add((row["object"], row["field"]))
    for api in APIS:
        for variant in ENHANCED_ORDER_VARIANTS:
            if not members.get((api, variant)):
                raise ValueError(f"enhanced order membership missing for {variant} at {api}")
    if members[(APIS[0], "feature-only")] != members[(APIS[1], "feature-only")]:
        raise ValueError("enhanced order baseline membership differs between API62 and API67")
    for api in APIS:
        if members[(api, "setting-only")] != members[(api, "feature-and-setting")]:
            raise ValueError(f"enhanced order setting membership depends on the feature at {api}")
        if members[(api, "setting-only")] != members[(api, "capitalized-settings")]:
            raise ValueError(f"enhanced order setting membership depends on the block spelling at {api}")
    added = [members[(api, "setting-only")] - members[(api, "feature-only")] for api in APIS]
    if any(members[(api, "feature-only")] - members[(api, "setting-only")] for api in APIS):
        raise ValueError("enhanced orders removed a baseline field")
    stable = added[0] & added[1]
    for required in (("Order", "OrderedDate"), ("OrderItem", "TotalLineAmount")):
        if required not in stable:
            raise ValueError(f"enhanced order gate lost {required[0]}.{required[1]}")
    rows = inputs.tsv(f"{prefix}/org.tsv")
    for api in APIS:
        for variant in ENHANCED_ORDER_VARIANTS:
            for case in ("order_ordereddate_ctor", "order_ordereddate_datetime", "order_ordereddate_read",
                         "orderitem_totallineamount_read", "orderitem_totallineamount_ctor"):
                matches = [row for row in rows if (row["api"], row["variant"], row["route"], row["case"]) ==
                           (api, variant, "deploy-compile", case)]
                want = "rejected" if variant == "feature-only" else "accepted"
                if len(matches) != 1 or matches[0]["outcome"] != want:
                    raise ValueError(f"enhanced order native compile row {variant}/{case} at {api}")
    described = inputs.json(f"{prefix}/granted-describe.json.gz")
    gates = []
    for obj, field in sorted(stable):
        facets = []
        for api in APIS:
            for variant in ("setting-only", "feature-and-setting"):
                facets.append(next(value for value in described[api][variant][obj] if value["name"] == field))
        if any(value != facets[0] for value in facets[1:]):
            raise ValueError(f"enhanced order facets for {obj}.{field} differ between captures")
        facet = dict(facets[0])
        if facet.get("picklistValues"):
            raise ValueError(f"enhanced order facets for {obj}.{field} carry picklist values")
        facet["restrictedPicklist"] = False
        gates.append({"object": obj, "features": [ENHANCED_ORDER_FEATURE], "field": facet})
    return gates


WEBSTORE_OPTION_FACETS = "BOOLEAN|ctrue|utrue|nfalse|dfltfalse|atrue"


def webstore_option_fields(inputs, profile_definition):
    """Return the WebStore Boolean fields with native describe, put, insert and
    select rows. The capture uses the P10 definition at both APIs; the facets
    must be createable, updateable, not nillable and not required on insert."""
    prefix = "../standard_webstore_options"
    definition = inputs.json(f"{prefix}/definition.json")
    for key in ("edition", "features", "settings"):
        if definition.get(key) != profile_definition.get(key):
            raise ValueError("WebStore option capture no longer uses the P10 definition")
    inputs.read(f"{prefix}/ORG-RUN.txt")
    rows = inputs.tsv(f"{prefix}/org.tsv")
    fields = None
    for api in APIS:
        by_case = {row["case"]: row for row in rows if row["api"] == api and row["route"] == "test-runner"}
        names = sorted(case.split(":", 1)[1] for case in by_case if case.startswith("members:"))
        if not names or fields not in (None, names):
            raise ValueError("WebStore option membership differs between API62 and API67")
        fields = names
        for name in names:
            if by_case[f"members:{name}"]["observation"] != WEBSTORE_OPTION_FACETS:
                raise ValueError(f"WebStore {name} native facets changed")
        if by_case["names_only:insert_name_only"]["observation"] != "OK":
            raise ValueError("WebStore Boolean options are no longer optional on insert")
        for case in ("preserve_unset_guestfalse", "preserve_unset_guesttrue",
                     "preserve_false_guestfalse", "preserve_false_guesttrue",
                     "preserve_true_guestfalse", "preserve_true_guesttrue"):
            if by_case[f"preserve:{case}"]["observation"] != "insert true,update_false false":
                raise ValueError(f"WebStore forced insert behavior changed: {case} at {api}")
        if not by_case["dyn_put_all_false:insert_all_false"]["observation"].startswith("OK "):
            raise ValueError("WebStore dynamic put of every option no longer inserts")
        for case, want in (("webstore_options_ctor", "accepted"), ("webstore_options_read", "accepted"),
                           ("webstore_options_assign", "accepted"), ("webstore_soql_options", "accepted"),
                           ("webstore_unknown_ctor_control", "rejected")):
            matches = [row for row in rows if row["api"] == api and row["route"] == "deploy-compile"
                       and row["case"] == case]
            if len(matches) != 1 or matches[0]["outcome"] != want:
                raise ValueError(f"WebStore option native compile row {case} at {api}")
    return fields


def commerce_entitlement_objects(inputs):
    """Fields of the WebStore link and entitlement objects with native describe,
    put, insert and select rows from the same P10 definition. Facets come from
    the captured REST describe and must agree at API62 and API67."""
    prefix = "../standard_webstore_options"
    rows = inputs.tsv(f"{prefix}/org.tsv")
    described = inputs.json(f"{prefix}/commerce-describe.json.gz")
    objects = {}
    for obj, fields in sorted(described[APIS[0]].items()):
        if fields != described[APIS[1]][obj]:
            raise ValueError(f"{obj} describe facets differ between API62 and API67")
        for api in APIS:
            by_case = {row["case"]: row["observation"] for row in rows
                       if row["api"] == api and row["route"] == "test-runner"}
            for field in fields:
                observed = by_case.get(f"ce_members:{obj}.{field['name']}")
                want = "|".join([field["type"].upper(), "c" + str(field["createable"]).lower(),
                                 "u" + str(field["updateable"]).lower(), "n" + str(field["nillable"]).lower(),
                                 "atrue"])
                if observed != want:
                    raise ValueError(f"{obj}.{field['name']} native describe row differs at {api}")
            for case in ("ce_chain:store_group_link", "ce_chain:policy_product", "ce_chain:policy_buyergroup"):
                if by_case.get(case) != "true,true":
                    raise ValueError(f"native entitlement chain row {case} at {api}")
            if by_case.get("ce_policy_name_only:policy_name_only") != "OK false,false,false":
                raise ValueError("entitlement policy booleans are no longer optional on insert")
        if any(field.get("picklistValues") for field in fields):
            raise ValueError(f"{obj} captured facets carry picklist values")
        names = [field["name"] for field in fields]
        objects[obj] = {"present": names, "captured_fields": names, "fields": fields}
    return objects


def build_policy(root, combinations=None):
    inputs = Inputs(root)
    combinations = Path(combinations) if combinations else inputs.root.parent / "standard_field_policy_combinations"
    combination_prefix = os.path.relpath(combinations, inputs.root)
    combination_rows = inputs.tsv(f"{combination_prefix}/org.tsv")
    inputs.read(f"{combination_prefix}/ORG-RUN.txt")
    validate_person_address_rows(combination_rows)
    for path in sorted(combinations.glob("*-definition.json")):
        inputs.json(os.path.relpath(path, inputs.root))
    inputs.json(f"{combination_prefix}/observations.json")

    plan = inputs.json("cases.json")
    field_rows = inputs.tsv("feature-set-differences.tsv")
    object_rows = inputs.tsv("global-object-differences.tsv")
    core_gates = core_gate_membership(field_rows)
    evidence = profile_evidence(field_rows, object_rows)
    snapshots = {}
    definitions = {}
    for profile in PROFILES:
        definitions[profile] = inputs.json(f"profiles/{profile}.json")
        inputs.read(f"{profile}/ORG-RUN.txt")
        snapshots[profile] = {api: snapshot(inputs, profile, api, set(plan["objects"])) for api in APIS}
    verify_profile_evidence(snapshots, evidence)
    api68 = api68_profile_evidence(inputs, snapshots, set(plan["objects"]))
    profile_aliases = {}
    for profile, captured in api68.items():
        if captured == evidence[profile]["67.0"]:
            profile_aliases[profile] = {"68.0": "67.0"}
        else:
            evidence[profile]["68.0"] = captured

    wanted = {}
    for row in plan["historical_extra_fields"]:
        wanted.setdefault(row["object"], set()).add(row["field"])
    for obj, fields in plan["samples"].items():
        wanted.setdefault(obj, set()).update(field for field in fields if field != "*")
    objects = {}
    for obj, targets in sorted(wanted.items()):
        baseline = snapshots["P0"][APIS[0]]
        if obj not in baseline["describes"]:
            continue
        present_by_api = [{field for name, field in membership(snapshots["P0"][api]) if name == obj} & targets
                          for api in APIS]
        if present_by_api[0] != present_by_api[1]:
            raise ValueError(f"baseline candidates for {obj} differ between API62 and API67")
        present = present_by_api[0]
        facets = {field["name"]: field for field in baseline["describes"][obj]["fields"]}
        objects[obj] = {"present": sorted(present), "absent": sorted(targets - present),
                        "fields": [facets[field] for field in sorted(present) if field in facets]}

    for family in ("standard_recordtype_field_policy", "standard_constructor_virtual_fields",
                   "standard_emailmessage_input_fields"):
        rows = inputs.tsv(f"../{family}/org.tsv")
        inputs.read(f"../{family}/ORG-RUN.txt")
        if family == "standard_recordtype_field_policy":
            require_rows(rows, None, profile="baseline", observation="MISSING")
            require_rows(rows, None, profile="recordtype", observation="REFERENCE|")
        else:
            case = "toids-list" if family == "standard_constructor_virtual_fields" else "toids-null-insert"
            require_rows(rows, case, observation="accepted")

    objects["EmailMessage"]["input_fields"] = ["ToIds"]
    for obj in ("Account", "EmailMessage", *qualified_constructor_objects(inputs)):
        fields = [constructor_fields(snapshots["P0"][api], obj, objects[obj].get("input_fields", []))
                  for api in APIS]
        if fields[0] != fields[1]:
            raise ValueError(f"{obj} constructor surface differs between API62 and API67")
        if obj in COMPLETE_CONSTRUCTOR_OBJECTS:
            excluded = set(objects[obj]["absent"]) | {
                field for gates in core_gates.values() for owner, field in gates if owner == obj}
            fields[0] = sorted(set(fields[0]) | preserved_profile_constructor_fields(snapshots, obj, excluded))
        objects[obj]["constructor_fields"] = fields[0]

    license_only = inputs.json("P10/license-only/profile-definition.json")
    if set(license_only.get("features", [])) != {"B2BCommerce", "Communities"}:
        raise ValueError("unexpected commerce license-only feature control")
    license_objects = set()
    for api in APIS:
        prefix = f"P10/license-only/api{api.split('.')[0]}"
        license_objects.update(inputs.json(f"{prefix}/granted/global.json")["objects"])
        if any(row["object"] == "WebStore" and row["object_present"] == "true"
               for row in inputs.tsv(f"{prefix}/default/named-fields.tsv")):
            raise ValueError("WebStore appears in license-only named control")
    if definitions["P8"].get("features") != ["OrderManagement"]:
        raise ValueError("OrderManagement profile no longer isolates its feature")
    if (set(definitions["P10"].get("features", [])) != {"B2BCommerce", "Communities"}
            or definitions["P10"].get("settings", {}).get("commerceSettings", {}).get("commerceEnabled") is not True):
        raise ValueError("commerce profile no longer pins captured enablement")
    require_rows(combination_rows, "constructor", profile="order-management", observation="accepted")
    require_rows(combination_rows, "missing-constructor", profile="order-management", compiled=False,
                 observation_contains="Field does not exist: UnknownOwnedField on WebStore")
    webstore_fields = set()
    for profile in ("P8", "P10"):
        fields = [constructor_fields(snapshots[profile][api], "WebStore") for api in APIS]
        if fields[0] != fields[1]:
            raise ValueError(f"WebStore {profile} constructor surface differs between API62 and API67")
        webstore_fields.update(fields[0])
    # Named P8 and enabled P10 both expose these fields. The P10 describe
    # supplies their API-stable Boolean facets even though P8 REST omits them.
    option_fields = webstore_option_fields(inputs, definitions["P10"])
    present, facet_list = [], []
    for name in option_fields:
        facets = [next(field for field in snapshots["P10"][api]["describes"]["WebStore"]["fields"]
                       if field["name"] == name) for api in APIS]
        if facets[0] != facets[1]:
            raise ValueError(f"WebStore {name} facets differ between API62 and API67")
        for profile in ("P8", "P10"):
            for api in APIS:
                if ("WebStore", name) not in membership(snapshots[profile][api]):
                    raise ValueError(f"WebStore {name} membership missing for {profile} at {api}")
        present.append(name)
        facet_list.append(facets[0])
    objects["WebStore"] = {
        "present": present,
        "captured_fields": present,
        "insert_forced_values": {"OptionsPreserveGuestCartEnabled": True},
        "fields": facet_list,
        "required_features_any_of": webstore_conditions(object_rows, license_objects),
        "constructor_fields": sorted(webstore_fields),
    }

    for obj, entry in commerce_entitlement_objects(inputs).items():
        if obj in objects:
            raise ValueError(f"{obj} is already a policy object")
        objects[obj] = entry

    # Explicit absence checks from a plain Developer org. Restrict these to
    # that exact declared shape; other editions and enabled features remain
    # unknown. Require the same checked subjects and absences on both APIs and
    # both profiles, with and without the marketing-user flag.
    shape_rows = inputs.tsv("../load_defaults_shape/org.tsv")
    inputs.read("../load_defaults_shape/ORG-RUN.txt")
    shape_groups = {}
    for row in shape_rows:
        if "." not in row["subject"]:
            continue
        group = (row["source_api"], row["profile"], row["marketing"])
        values = shape_groups.setdefault(group, {})
        if row["subject"] in values:
            raise ValueError("duplicate native field shape observation")
        values[row["subject"]] = row["observation"].split("|", 1)[0]
    expected_groups = {(api, profile, flag) for api in APIS
                       for profile in ("admin", "standard") for flag in ("true", "false")}
    if set(shape_groups) != expected_groups:
        raise ValueError("incomplete native plain Developer field shape")
    baseline = shape_groups[("62.0", "admin", "false")]
    if not baseline or any(group != baseline for group in shape_groups.values()):
        raise ValueError("plain Developer field membership differs across controls")
    for subject, observation in sorted(baseline.items()):
        if observation == "exists=false":
            obj, field = subject.split(".", 1)
            entry = objects.setdefault(obj, {"present": [], "absent": [], "fields": []})
            if field in entry.get("present", []):
                raise ValueError(f"conflicting plain Developer field membership: {subject}")
            entry.setdefault("plain_developer_absent", []).append(field)
        elif observation != "exists=true":
            raise ValueError(f"invalid native field membership: {subject}")

    # A populated, defaulted field also needs its captured schema facets. Use
    # the separately captured granted P0 describes, and require the full facet
    # record to agree at both API endpoints before enriching the policy.
    for row in shape_rows:
        if row["source_api"] != "62.0" or row["profile"] != "admin" or row["marketing"] != "false" or "." not in row["subject"]:
            continue
        facets = dict(item.split("=", 1) for item in row["observation"].split("|"))
        if facets.get("exists") != "true" or facets.get("default") != "true" or facets.get("pop") != "true":
            continue
        obj, field = row["subject"].split(".", 1)
        captured = []
        for api in APIS:
            describe = snapshots["P0"][api]["describes"].get(obj, {})
            matches = [item for item in describe.get("fields", []) if item["name"] == field]
            if len(matches) != 1:
                raise ValueError(f"missing plain Developer default facets: {obj}.{field} {api}")
            captured.append(matches[0])
        if captured[0] != captured[1] or not captured[0].get("defaultedOnCreate"):
            raise ValueError(f"plain Developer default facets differ: {obj}.{field}")
        entry = objects.setdefault(obj, {"present": [], "absent": [], "fields": []})
        if field not in entry.get("present", []):
            entry.setdefault("present", []).append(field)
            entry["present"].sort()
        if not any(item["name"] == field for item in entry.get("fields", [])):
            entry.setdefault("fields", []).append(captured[0])
            entry["fields"].sort(key=lambda item: item["name"])

    # Some constructor-populated values are not describe defaults. Keep them
    # in a separate plain-Developer constructor policy rather than rewriting
    # the public describe facets. Require all captured profiles, flag states,
    # APIs, and the false-overload control to agree.
    value_rows = inputs.tsv("../load_defaults_values/org.tsv")
    inputs.read("../load_defaults_values/ORG-RUN.txt")
    values = {}
    for row in value_rows:
        key = (row["source_api"], row["profile"], row["marketing"])
        group = values.setdefault(row["subject"], {})
        if key in group:
            raise ValueError("duplicate constructor value observation")
        group[key] = dict(item.split("=", 1) for item in row["observation"].split("|"))
    expected_values = {(api, profile, marketing) for api in APIS
                       for profile, marketing in (("admin", "false"), ("admin", "true"), ("standard", "false"))}
    for subject, group in sorted(values.items()):
        if set(group) != expected_values or any(v != next(iter(group.values())) for v in group.values()):
            raise ValueError(f"constructor values differ across controls: {subject}")
        observed = next(iter(group.values()))
        if (observed.get("loadedPop"), observed.get("blank"), observed.get("blankPop")) != ("true", "null", "false"):
            raise ValueError(f"constructor value boundary differs: {subject}")
        obj, field = subject.split(".", 1)
        facets = []
        for api in APIS:
            describe = snapshots["P0"][api]["describes"].get(obj, {})
            matches = [item for item in describe.get("fields", []) if item["name"] == field]
            if len(matches) != 1:
                raise ValueError(f"missing constructor field facets: {subject} {api}")
            facets.append(matches[0])
        if facets[0] != facets[1]:
            raise ValueError(f"constructor field facets differ by API: {subject}")
        if facets[0].get("defaultValue") is not None:
            continue
        value = json.loads(observed["loaded"])
        if not isinstance(value, (int, float)) or isinstance(value, bool):
            raise ValueError(f"unsupported constructor default value: {subject}")
        entry = objects.setdefault(obj, {"present": [], "absent": [], "fields": []})
        entry.setdefault("constructor_defaults", {})[field] = observed["loaded"]

    # Some inaccessible internal fields are reported as removed on original
    # loaded records even though describe does not expose them. Derive that
    # set by subtracting clone and fresh controls from both profile routes.
    hidden_rows = inputs.tsv("../load_defaults_hidden_routes/org.tsv")
    inputs.read("../load_defaults_hidden_routes/ORG-RUN.txt")
    hidden_groups = {}
    for row in hidden_rows:
        key = (row["source_api"], row["profile"], row["subject"], row["route"])
        if key in hidden_groups or not row["observation"].startswith("removed="):
            raise ValueError("invalid hidden-field route observation")
        hidden_groups[key] = set(filter(None, row["observation"][len("removed="):].split(",")))
    for obj in sorted({row["subject"] for row in hidden_rows}):
        candidates = []
        for api in APIS:
            for profile in ("admin", "standard"):
                routes = {route: hidden_groups[(api, profile, obj, route)]
                          for route in ("plain", "false", "true", "changed", "cloned")}
                hidden = routes["true"] - routes["cloned"]
                if hidden != routes["changed"] - routes["cloned"] or hidden & (routes["plain"] | routes["false"]):
                    raise ValueError(f"hidden-field routes disagree: {obj} {api} {profile}")
                candidates.append(hidden)
        if not candidates[0] or any(fields != candidates[0] for fields in candidates):
            raise ValueError(f"hidden-field controls disagree: {obj}")
        entry = objects.setdefault(obj, {"present": [], "absent": [], "fields": []})
        if not candidates[0].issubset(set(entry.get("plain_developer_absent", []))):
            raise ValueError(f"hidden fields lack describe-absence evidence: {obj}")
        entry["constructor_hidden_removed"] = sorted(candidates[0])

    # Each isolated profile proves a positive field and controls show it absent
    # without the enabling setting. The combined profile supplies both Apex
    # routes and independently captured REST facets at both API endpoints.
    family = "../standard_industries_schema"
    industries_rows = inputs.tsv(f"{family}/org.tsv")
    boundary_rows = inputs.tsv(f"{family}/boundary.tsv")
    inputs.read(f"{family}/ORG-RUN.txt")
    inputs.read(f"{family}/SchemaNativeTest.cls")
    license_profile = inputs.json(f"{family}/license-profile.json")
    community_profile = inputs.json(f"{family}/communities-profile.json")
    if set(license_profile.get("features", [])) != {"B2BCommerce", "Communities"} or license_profile.get("settings", {}).get("commerceSettings", {}).get("commerceEnabled") is True:
        raise ValueError("unexpected commerce license-only control")
    if set(community_profile.get("features", [])) != {"Communities"} or community_profile.get("settings", {}).get("commerceSettings", {}).get("commerceEnabled") is not True:
        raise ValueError("unexpected communities-only provisioning control")
    new_gates = []
    for obj, name, profile, controls, features in (
        ("AccountContactRelation", "Roles", "contacts", ("industry", "commerce"), ["ContactsToMultipleAccounts"]),
        ("PricebookEntry", "ProductSellingModelId", "commerce", ("industry", "contacts", "license"), ["B2BCommerce", "Communities", "CommerceEnabled"]),
        ("User", "MiddleName", "name", ("industry", "commerce"), ["MiddleNameEnabled"]),
    ):
        field_name = f"{obj}.{name}"
        facets = []
        for api in APIS:
            value = inputs.json(f"{family}/{api}-{obj}-{name}.json")
            expected = f"{value['type'].upper()}|{str(value['nillable']).lower()}|{str(value['defaultedOnCreate']).lower()}"
            for profile_name, route in ((f"isolated-{profile}", "anonymous"), ("combined", "anonymous"), ("combined", "named"), ("combined", "test-runner")):
                rows = [row for row in industries_rows if row["profile"] == profile_name and row["api"] == api and row["route"] == route and row["case"] == field_name]
                if len(rows) != 1 or rows[0]["result"] != expected:
                    raise ValueError(f"missing native field result: {profile_name} {api} {route} {field_name}")
            for control in controls:
                rows = [row for row in industries_rows if row["profile"] == f"isolated-{control}" and row["api"] == api and row["route"] == "anonymous" and row["case"] == field_name]
                # Original industry and commerce profiles are also controls.
                if not rows:
                    rows = [row for row in industries_rows if row["profile"] == control and row["api"] == api and row["route"] == "anonymous" and row["case"] == field_name]
                if len(rows) != 1 or rows[0]["result"] not in ("field_absent", "object_absent"):
                    raise ValueError(f"missing native profile control: {control} {api} {field_name}")
            for route in ("anonymous", "named", "test-runner"):
                rows = [row for row in boundary_rows if row["api"] == api and row["route"] == route and row["case"] == obj]
                if len(rows) != 1 or rows[0]["observation"] != "true|false":
                    raise ValueError(f"missing native boundary: {api} {route} {obj}")
            facets.append(value)
        if facets[0] != facets[1]:
            raise ValueError(f"native field facets differ by API: {field_name}")
        gate = {"object": obj, "features": features, "field": facets[0]}
        if (obj, name) == ("AccountContactRelation", "Roles"):
            gate["override_stub_type"] = True
        new_gates.append(gate)

    gates = list(new_gates)
    gates.append(case_source_gate(inputs, snapshots))
    for profile, fields in core_gates.items():
        for obj, field in fields:
            facets = next(value for value in snapshots[profile][APIS[0]]["describes"][obj]["fields"]
                          if value["name"] == field)
            gate = {"object": obj, "features": [CORE_FEATURES[profile]], "field": facets}
            if (obj, field) == ("Account", "RecordTypeId"):
                gate["record_types"] = True
            gates.append(gate)
    for name in PERSON_ADDRESS_FIELDS:
        # The isolated combination rows prove availability. PU supplies only the
        # rich field facets; it is never used to infer a feature attribution.
        facets = next(field for field in snapshots["PU"][APIS[0]]["describes"]["Account"]["fields"]
                      if field["name"] == name)
        gates.append({"object": "Account", "features": ["PersonAccounts", "StateAndCountryPicklist"],
                      "field": facets})
    gates.extend(enhanced_order_gates(inputs))
    # The enabled Commerce profile adds this PricebookEntry field over
    # the otherwise identical license-only control. Keep this narrow to the
    # captured feature and setting combination and require stable facets.
    commerce_rows = inputs.tsv("P10/enablement-differences.tsv")
    commerce_features = ["B2BCommerce", "Communities", "CommerceEnabled"]
    for field in ("ActivePriceAdjustmentQuantity", "ProductSellingModelId"):
        for api in APIS:
            for route in ("anonymous-default", "named-default", "effective-membership"):
                if not any(row["api"] == api and row["route"] == route
                           and row["object"] == "PricebookEntry" and row["field"] == field
                           and row["change"] == "added" for row in commerce_rows):
                    raise ValueError(f"missing Commerce enablement control for {field} {api} {route}")
        facets = [next(value for value in snapshots["P10"][api]["describes"]["PricebookEntry"]["fields"]
                       if value["name"] == field) for api in APIS]
        if facets[0] != facets[1]:
            raise ValueError(f"Commerce field facets differ across APIs: {field}")
        if not any(gate["object"] == "PricebookEntry" and gate["field"]["name"] == field
                   for gate in gates):
            gates.append({"object": "PricebookEntry", "features": commerce_features, "field": facets[0]})
    # Preserve existing gates without expanding their metadata or attribution.
    # Settings-group and HealthCloud observations remain separate evidence.
    for obj, field, feature in (("Account", "IsActive", "HealthCloud"),
                                ("Opportunity", "SyncedQuoteId", "Quotes")):
        gates.append({"object": obj, "features": [feature], "field": {"name": field}, "legacy": True})
    return {"version": 1, "objects": objects,
            "gates": sorted(gates, key=lambda gate: (gate["object"], gate["field"]["name"])),
            "sources": inputs.sources,
            "evidence": {"profiles": evidence, "profile_aliases": profile_aliases,
                         "runtime_field_profiles": list(CORE_FEATURES),
                         "deferred_field_profiles": ["P4", "P5", "P6", "P7", "P8", "P9", "P10"],
                         "edition_control": "P11", "union_control": "PU"}}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--captures", type=Path, default=DEFAULT_CAPTURES)
    parser.add_argument("--combinations", type=Path)
    parser.add_argument("--output", type=Path, default=DEFAULT_OUTPUT)
    parser.add_argument("--check", action="store_true", help="verify the output without rewriting it")
    args = parser.parse_args()
    try:
        policy = build_policy(args.captures, args.combinations)
        data = policy_bytes(policy)
        if args.check:
            if args.output.read_bytes() != data:
                parser.exit(1, f"{args.output}: generated policy differs\n")
        else:
            args.output.write_bytes(data)
    except (OSError, ValueError, KeyError, StopIteration) as error:
        parser.exit(1, f"field policy generation failed: {error}\n")
    print(f"{'verified' if args.check else 'wrote'} {len(policy['objects'])} objects, "
          f"{len(policy['gates'])} gates, {len(policy['sources'])} source digests")


if __name__ == "__main__":
    main()
