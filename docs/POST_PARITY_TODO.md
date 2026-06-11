# Post-Parity TODO

## Performance Scanner And Bottleneck Reports

- [x] Move advisory project scanning to the performance plugin:
  `glade plugins install performance`, then `glade performance scan`.
- [x] Report entry points for triggers, batches, queueables, schedulables,
  invocable methods, Visualforce page actions, Aura server actions, LWC Apex
  imports/wires, Flow, and Workflow.
- [x] Flag static Salesforce-shaped risk: SOQL/DML/describe/async work in
  loops, unfiltered batch start queries, repeated describe, uncached
  `@AuraEnabled` reads, active Workflow, and Flow data fanout.
- [x] Add async chain analysis for queueable, batch, and schedule paths, including cycle/depth risk findings.
- [x] Accept local trace input and merge measured spans into the same ranked
  report.
- [x] Extend profile reports with trace duration attribution.
- [ ] Add optional org-backed SOQL query-plan enrichment after the local scanner
  is stable.
- [ ] Add SARIF output after JSON and Markdown stabilize.
