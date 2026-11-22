Native rows: `internal/apextest/testdata/conformance/custom_manual_share_lifecycle/with_read/org.tsv`

Native rows: sibling `org.tsv`. Capture details and source digests: sibling `RUN-FACTS.txt`.

Captured 2026-11-22T02:58:08.319870+00:00 in a fresh 30-day Developer scratch org.
Private synthetic custom object; original administrator owns each record; standard user has an explicit object read permission set and receives and loses a Manual Read share.
Each setup, share insertion, share deletion and record deletion executes as a separate committed anonymous Apex request. Each observation runs in a later request. APIs 62.0, 67.0 and 68.0, set with the sf apex run --api-version flag.
Outcome format: row count:read flag,edit flag,maximum access; 0:none means no row.
Deletion of the scratch org was verified. One run per API and control; anonymous route only.
source_sha256:object-meta.xml=5b4e9a4bb9e0fc5800305b5cdca5df0d3669b17b12e70743e12669b65199af02
source_sha256:read-permission-meta.xml=838a233d382af8b9486a9afe86ce6a86917cdc76bdc609139247db08f6dfa568
template_sha256:steps.txt=e1240b1a71e790c97fe4fca8c93737afabc029e67d93d2b6251fbd4dd7499960
