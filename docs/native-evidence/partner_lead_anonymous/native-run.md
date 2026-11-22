Native rows: `internal/vm/testdata/partner_lead_anonymous/org.tsv`

# Native capture

- Capture time is not recorded in the retained facts.

- APIs in retained rows: 62.0, 67.0.
- Routes in retained rows: anonymous.
- Retained org.tsv SHA-256: 39c72c60b0bce000b382df399b257cdbe896ca6a48a27f1402e11595ac04011a.

The retained capture and cleanup facts are in [RUN-FACTS.txt](../../../internal/vm/testdata/partner_lead_anonymous/RUN-FACTS.txt).

The following source hashes were computed from the retained files after capture; they are not contemporaneous capture receipts. Project snapshots or probe literals were compared byte for byte with the fixture.

- `query.apex` retained source SHA-256: `1deff1e3acbf29c75fa73a3f6704387ae05deefce826061a0846cfca686fa990`; it matches a retained probe source literal.
- Retained probe script SHA-256: `d4ffc7bf5d814a5801e563d8aeb7be134e845f213184f6eadece863f8a1f9822`; computed after capture, not a capture-time digest.
- A retained sanitized observation artifact has 4 rows matching the fixture by API and outcome; its filesystem modification time is 2026-11-22T07:38:58Z. This is an artifact timestamp, not a Salesforce observation timestamp.
