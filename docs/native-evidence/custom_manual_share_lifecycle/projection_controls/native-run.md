Native rows: `internal/apextest/testdata/conformance/custom_manual_share_lifecycle/projection_controls/org.tsv`

Native rows: sibling `org.tsv`. Capture details and source digests: sibling `RUN-FACTS.txt`.

Captured 2026-11-22T03:48:48.198595+00:00 in a fresh 30-day Developer scratch org.
Private synthetic custom object; original administrator owns each record; standard user has an explicit object read permission set. No share is granted.
Each setup and record insertion executes as a separate committed anonymous Apex request; each observation runs in a later request. API 68.0 selects RecordId and HasReadAccess. API 67.0 also selects HasEditAccess.
Each API was set with the sf apex run --api-version flag. The org was deleted and deletion verified. One observation per API.
source_sha256:object-meta.xml=5b4e9a4bb9e0fc5800305b5cdca5df0d3669b17b12e70743e12669b65199af02
source_sha256:read-permission-meta.xml=838a233d382af8b9486a9afe86ce6a86917cdc76bdc609139247db08f6dfa568
template_sha256:steps.txt=d44fadf1a2438714d47a453801974977c8c0d5b22351b30223b58a49e51aaae9

- APIs in retained rows: 67.0, 68.0; routes: anonymous.
- Retained org.tsv SHA-256: 08b00253531e17fb047441a342363a0342a4fb7e90ea798463df8edc9a77f44a.
- Cleanup facts: [RUN-FACTS.txt](../../../../internal/apextest/testdata/conformance/custom_manual_share_lifecycle/projection_controls/RUN-FACTS.txt).
