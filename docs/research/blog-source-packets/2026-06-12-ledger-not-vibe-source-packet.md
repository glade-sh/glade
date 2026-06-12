# Source Packet: The Ledger, Not the Vibe

Generated: 2026-06-12

Post in series: **Building Glade With AI**

Working title: **The Ledger, Not the Vibe**

Core claim: The strongest AI-assisted Glade work used a fresh machine-readable ledger as truth. The work did not end when the agent said it was done. It ended when the final ledger and checks showed exact movement.

## Publication Guardrails

- Public-safe topic. No private package names needed.
- Do not mention early internal project names, internal repo paths, or internal module paths.
- Treat this as Glade history.
- Historical fixture paths listed below came from the rollout record. They are not present in the current base Glade checkout after the plugin split.
- Avoid long transcript excerpts. Use short quotes and measured facts.

## Primary Sources

- Memory rollout summary: `/Users/matt/.codex/memories/rollout_summaries/2026-06-08T13-28-49-k3IK-glade_surface_ledger_parallel_vertical_closeout.md`
- Series plan: `docs/research/ai-assisted-glade-blog-series-plan.md`
- Source index: `docs/research/ai-assisted-glade-blog-source-index.md`

## Short Quotes To Use

- `Use that temp SURFACE_LEDGER.json as truth.`
- `This sprint MUST use parallel subagent squads to move faster.`
- `Use explicitUnsupported fixtures only for true external/server-only/product surfaces.`
- `Confirm target vertical gap counts dropped or are zero.`

## Timeline

| Step | Fact |
| --- | --- |
| 1 | Build `/tmp/glade`. |
| 2 | Run a fresh `compat surface refresh` into `/tmp/glade-surface-20260608-062939`. |
| 3 | Use `/tmp/glade-surface-20260608-062939/SURFACE_LEDGER.json` as baseline truth. |
| 4 | Split work into three parallel squads: Platform.Events; GraphQL/PubSub; Salesforce Connect Amazon RDS, AMPscript, Handlebars, Agentforce. |
| 5 | Add seven distinct explicit-unsupported fixtures. |
| 6 | Run `compat validate`, `compat run`, focused fixture tests, `go test` for surface ledger, and `go test` for repo guard. |
| 7 | Run a fresh final surface refresh. |
| 8 | Pass the final measured check with `--max-missing-shape 6774`. |

## Ledger Counts

Baseline from fresh refresh:

| Metric | Baseline |
| --- | ---: |
| implemented | 129349 |
| partial | 30 |
| passive | 47578 |
| stubNoOp | 318 |
| explicitUnsupported | 1047 |
| missingShape | 6838 |
| missingBehavior | 0 |
| missingEvidence | 4838 |

Final movement:

| Metric | Before | After | Move |
| --- | ---: | ---: | ---: |
| explicitUnsupported | 1047 | 1111 | +64 |
| missingShape | 6838 | 6774 | -64 |

Important nuance:

- The final strict check needed a measured missing-shape ceiling.
- That was honest. The sprint moved its packet. It did not claim the whole repo-wide surface debt was gone.

## Fixture Evidence

Historical fixture names from the rollout record:

- `docs/fixtures/platform-events-metadata-tooling-unsupported.json`
- `docs/fixtures/integration-graphql-api-explicit-unsupported.json`
- `docs/fixtures/integration-pubsub-api-explicit-unsupported.json`
- `docs/fixtures/integration-salesforce-connect-amazon-rds-unsupported.json`
- `docs/fixtures/external-marketing-cloud-ampscript-unsupported.json`
- `docs/fixtures/external-marketing-cloud-handlebars-unsupported.json`
- `docs/fixtures/ai-agentforce-product-surfaces-unsupported.json`

What moved:

- Platform.Events moved 11 rows.
- GraphQL moved 5 rows.
- PubSub moved 7 rows.
- Salesforce Connect Amazon RDS moved 2 rows.
- AMPscript moved 17 rows.
- Handlebars moved 10 rows.
- Agentforce moved 14 rows.

Total: 64 rows.

## Validation Evidence

Focused fixture test from the rollout record:

```bash
go test -count=1 -timeout=120s ./internal/compat -run 'TestRunDocumentedFixtures/(platform-events-metadata-tooling-unsupported|integration-graphql-api-explicit-unsupported|integration-pubsub-api-explicit-unsupported|integration-salesforce-connect-amazon-rds-unsupported|external-marketing-cloud-ampscript-unsupported|external-marketing-cloud-handlebars-unsupported|ai-agentforce-product-surfaces-unsupported)'
```

Other passing checks from the rollout record:

```bash
go test -count=1 -timeout=120s ./internal/surfaceledger
go test -count=1 -timeout=120s ./internal/repoguard
```

Final measured check:

```bash
compat surface check --ledger /tmp/glade-surface-final-20260608-065253/SURFACE_LEDGER.json --max-parser-failures 0 --max-missing-shape 6774
```

## Draft Outline

1. Open with the artifact: `SURFACE_LEDGER.json`.
2. Explain why a fresh ledger mattered more than a stale report or an agent summary.
3. Show the rules: fresh build, fresh refresh, parallel squads, explicit unsupported only for true external/server-only/product surfaces.
4. Show the work: three squads, seven fixtures, validation.
5. Show the movement: `missingShape 6838 -> 6774`, `explicitUnsupported 1047 -> 1111`.
6. Show the honest limit: the whole surface debt was not zero.
7. End with the next artifact: a source packet and draft for the next post, or the next ledger run.

## Chart Idea

Simple two-bar before/after chart:

```text
missingShape          6838 -> 6774   -64
explicitUnsupported   1047 -> 1111   +64
```

## Drafting Notes

- Lead with facts, not a claim about AI.
- Keep the human role concrete: set the gate, set the boundary, demand the final counts.
- Keep the AI role concrete: split work, inspect packets, add fixtures, run checks, integrate results.
- The lesson is not "AI can do anything." The lesson is "AI work gets better when the artifact is sharper than the answer."
