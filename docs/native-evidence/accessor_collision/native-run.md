Salesforce Developer scratch org; capture date 2026-11-09.
Deployed the same collision and setter-read classes separately at API 62.0 and 67.0 and ran named tests. Own-setter read asserted the getter result but failed with the recorded actual null; a separate backing-read class at each API asserted null and zero getter invocations and passed.
