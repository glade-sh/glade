Native rows: `org.tsv`

Captured 2026-11-22 from 01:44:06Z to 01:46:01Z in a 30-day scratch org, deleted after the run. The live capture used Salesforce CLI 2.157.8 and capture-v7 in rename mode. The request contained four cases and ran at APIs 62.0 and 67.0 through anonymous and isTest routes. `org.tsv` contains selected normalized rows from the eight-row capture.

Anonymous probes rolled back role and account work, asserted that the role count was restored, and created a tracked cleanup record. The capture checked and cleaned records in Account and PermissionSet; its recorded cleanup limitations and residue results are in `RUN-FACTS.txt`. The replay is a parity pin and passed on the unchanged origin/main runtime.
