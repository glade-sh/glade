#!/usr/bin/env python3
"""Recreate the anonymous DML probes used for the captured locale rows.

Run with an output directory on a work volume, then execute each generated
file once at API 62.0 and once at API 67.0 against the same owned scratch org.
The describe-and-edge.apex capture supplies the API 67 entries in org.tsv.
"""
import json
from pathlib import Path
import sys

rows = Path(__file__).with_name("org.tsv").read_text().splitlines()
entries = json.loads(next(row.split("\t", 2)[2] for row in rows[1:]
                          if row.startswith("67.0\t")))["entries"]
values = [entry["value"] for entry in entries]
collections = {
    "batch": values + ["EN_US", "en_us", "Af", "af_ZA", "xx_XX", "en-GB"],
    "prefix": sorted({value.split("_")[0] for value in values})
              + ["en_FOO", "en_XY", "zz_US"],
}
out = Path(sys.argv[1])
out.mkdir(parents=True, exist_ok=True)
for name, items in collections.items():
    for offset in range(0, len(items), 90):
        values_apex = ",".join("'" + item.replace("'", "\\'") + "'"
                               for item in items[offset:offset + 90])
        source = (
            "Savepoint sp=Database.setSavepoint();\n"
            "for(String v : new List<String>{" + values_apex + "}) { "
            "Database.SaveResult r=Database.update(new User("
            "Id=UserInfo.getUserId(),LocaleSidKey=v),false); "
            "System.debug('DML_ROW|' + JSON.serialize(new List<Object>"
            "{v,r.isSuccess()})); }\nDatabase.rollback(sp);\n"
        )
        (out / f"{name}{offset // 90}.apex").write_text(source)
