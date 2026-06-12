# The Ledger, Not the Vibe

The first useful artifact for this story is not a model transcript.

It is a JSON file.

On June 8, 2026, Glade had a broad Salesforce surface problem. The project knew a great deal. It also had a long list of things it did not know yet. Some were real runtime gaps. Some were server-only APIs. Some were product surfaces that should never be faked by a local Apex runtime.

That is dangerous ground for AI work. A model can make a tidy list. It can write a confident closeout. It can find a few green tests and call the job done.

So the instruction was plain:

`Use that temp SURFACE_LEDGER.json as truth.`

Not the previous report. Not the plan. Not the agent's summary. The ledger.

## The Trailhead

The sprint started with a clean sequence.

First, build `/tmp/glade`.

Then run a fresh `compat surface refresh`.

Then use the new `SURFACE_LEDGER.json` as the baseline. The path for that run was:

`/tmp/glade-surface-20260608-062939/SURFACE_LEDGER.json`

That file gave the starting shape:

| Metric | Count |
| --- | ---: |
| implemented | 129349 |
| partial | 30 |
| passive | 47578 |
| stubNoOp | 318 |
| explicitUnsupported | 1047 |
| missingShape | 6838 |
| missingBehavior | 0 |
| missingEvidence | 4838 |

There was the cut line. No guessing.

The target work was narrow enough to hand to parallel agents and broad enough to matter. Platform Events. GraphQL. PubSub. Salesforce Connect Amazon RDS. AMPscript. Handlebars. Agentforce product surfaces.

The rule was just as important as the list:

`Use explicitUnsupported fixtures only for true external/server-only/product surfaces.`

That sentence did a lot of work. It stopped the sprint from using unsupported as a junk drawer. If Glade should model the behavior locally, then the fix belonged in the runtime. If the surface depended on a Salesforce server, an external product, or a cloud-only API, the right answer was an explicit fixture that said so.

## Three Squads

The work split into three squads.

One took Platform Events.

One took GraphQL and PubSub.

One took the external and product surfaces: Salesforce Connect Amazon RDS, AMPscript, Handlebars, and Agentforce.

The main thread stayed on integration. That was the right shape for this kind of job. The squads could inspect packet summaries and prepare fixture evidence. The main thread could keep the ledger, validation, and final counts straight.

Each touched row needed a reason. The workflow used `compat surface packet` to find the work and `compat surface explain` to decide whether the row was a runtime gap or a true unsupported surface.

That matters. Without that step, a sprint like this turns into labeling. With it, each row gets handled like evidence.

## Seven Fixtures

The result was seven explicit-unsupported fixtures. These paths are historical now. Base Glade later moved maintenance fixtures toward the plugin side. At the time, the sprint record listed them this way:

```text
docs/fixtures/platform-events-metadata-tooling-unsupported.json
docs/fixtures/integration-graphql-api-explicit-unsupported.json
docs/fixtures/integration-pubsub-api-explicit-unsupported.json
docs/fixtures/integration-salesforce-connect-amazon-rds-unsupported.json
docs/fixtures/external-marketing-cloud-ampscript-unsupported.json
docs/fixtures/external-marketing-cloud-handlebars-unsupported.json
docs/fixtures/ai-agentforce-product-surfaces-unsupported.json
```

They were not broad exceptions. They were named pieces of evidence.

The row movement was concrete:

| Area | Rows moved |
| --- | ---: |
| Platform.Events | 11 |
| GraphQL | 5 |
| PubSub | 7 |
| Salesforce Connect Amazon RDS | 2 |
| AMPscript | 17 |
| Handlebars | 10 |
| Agentforce | 14 |
| Total | 64 |

Sixty-four rows moved from `missingShape` to `explicitUnsupported`.

Not bad.

## The Checks

Fixtures are not proof by themselves. They have to parse. They have to run. The code that reads them has to agree.

The sprint ran the focused fixture test:

```bash
go test -count=1 -timeout=120s ./internal/compat -run 'TestRunDocumentedFixtures/(platform-events-metadata-tooling-unsupported|integration-graphql-api-explicit-unsupported|integration-pubsub-api-explicit-unsupported|integration-salesforce-connect-amazon-rds-unsupported|external-marketing-cloud-ampscript-unsupported|external-marketing-cloud-handlebars-unsupported|ai-agentforce-product-surfaces-unsupported)'
```

It also ran the surface-ledger and repo-guard tests:

```bash
go test -count=1 -timeout=120s ./internal/surfaceledger
go test -count=1 -timeout=120s ./internal/repoguard
```

Then came the important part. Run the refresh again.

The final refresh produced:

| Metric | Before | After | Change |
| --- | ---: | ---: | ---: |
| explicitUnsupported | 1047 | 1111 | +64 |
| missingShape | 6838 | 6774 | -64 |

The arithmetic balanced. Sixty-four rows went out of one bucket and into the other.

## The Honest Ceiling

The final strict check did not pretend the whole world was fixed.

It passed like this:

```bash
compat surface check --ledger /tmp/glade-surface-final-20260608-065253/SURFACE_LEDGER.json --max-parser-failures 0 --max-missing-shape 6774
```

That `6774` matters.

The sprint did not make missing shape debt disappear. It moved a defined packet and set the new measured ceiling. The broad repo still had work left. The sprint knew it. The command knew it. The report said it.

That is a better finish than a green sentence in a chat window.

## What AI Did

AI helped because the job had rails.

It could read packet summaries. It could inspect surface explanations. It could write fixtures. It could run validation. It could keep separate lanes moving at the same time.

But the useful work came from the shape of the task.

Use a fresh ledger.

Treat it as truth.

Split only clean verticals.

Use explicit unsupported only when unsupported is the honest answer.

Run the final refresh.

Write down the counts.

That is how the work stayed grounded.

## What The Human Did

The human work was not typing every line.

It was setting the test of truth before the agents started. It was choosing the boundary. It was saying no to fake local support. It was demanding the final count instead of accepting the story.

That is the part I would repeat.

Make the artifact sharper than the answer.

For this sprint, the artifact was a ledger. It had a path, a date, and numbers that had to move. The agents could work inside that frame. The final check could prove what happened.

The work ended with a new ceiling:

```text
missingShape          6838 -> 6774
explicitUnsupported   1047 -> 1111
```

Sixty-four rows. Seven fixtures. Three squads. One fresh ledger before. One fresh ledger after.

That was the day.
