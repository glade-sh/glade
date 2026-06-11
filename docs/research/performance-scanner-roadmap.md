# Performance Scanner Roadmap

The first-party performance plugin is useful when it finds work a developer can
actually reduce. Entry points are map pins. Findings should be bottlenecks.

## Current Contract

- Apex static findings cover high-confidence code shape: SOQL, DML, async, and
  describe work inside loops; repeated describe calls; broad SOQL projection;
  parent-child subqueries without a limit; rough selectivity risks; formula
  predicates; async chain and cycle risks.
- Metadata scanning records Visualforce, Aura, LWC, Flow, and Workflow entry
  points. It does not report UI entry points as findings by itself.
- Trace ingestion reports measured spans and measured SOQL row counts. A traced
  path should outrank static guesses.
- SOQL without a `WHERE` clause is not a finding by itself. The scanner needs
  row counts, selectivity evidence, projection cost, loop context, or measured
  duration before it calls a query a bottleneck.

## Near Work

1. Add first-class trace capture for common local-test and UI workflows.
   Visualforce, Aura, LWC, batch, scheduled, queueable, webhook, and trigger
   entry points should be easy to invoke and inspect.
2. Build an entry-point view that groups findings and measured spans under the
   trigger, batch, invocable, UI controller, flow, or test method that reached
   them.
3. Add delta comparison for CI and code review. Compare a current report with a
   saved baseline and show only new or worsened findings.
4. Add optional org-backed query-plan enrichment later. Keep it separate from
   local test runs and never require an org for the default scanner.

## Rules For Future Findings

- Prefer measured evidence over static evidence.
- Keep static findings narrow and explainable from source alone.
- Do not report entry points as defects.
- Do not report generic SOQL shape without row, selectivity, projection, loop,
  or measured-duration evidence.
- Do not add project-specific suppressions or special cases.
