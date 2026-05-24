# Oracle Parity

`glade compat oracle-tests` compares Salesforce observations with GLADE observations.
The pass/fail input is normalized JSON, not raw debug log text.

Current skeleton:

- normalizes IDs, timestamps, generated usernames, stack line noise, records, side effects, and trace events
- parses Apex logs for method calls, SOQL, DML, exceptions, limits, `USER_DEBUG`, and `GLADE_ORACLE:` payloads
- can opt into recent Apex log capture for focused Salesforce test runs with `--fetch-logs`
- adapts GLADE `testreport.Run` results into `OracleRun`
- diffs fixture or live runner output and writes compact artifacts under `.glade/runs/<run-id>/oracle/`

Fixture diff:

```bash
go run ./cmd/glade compat oracle-tests \
  --salesforce-run docs/fixtures/oracle/passing-test-salesforce.json \
  --local-run docs/fixtures/oracle/passing-test-local.json \
  --json
```

Forced trace mismatch:

```bash
go run ./cmd/glade compat oracle-tests \
  --salesforce-run docs/fixtures/oracle/passing-test-salesforce.json \
  --local-run docs/fixtures/oracle/trace-mismatch-local.json \
  --json
```

Checked fixture corpus:

```bash
go run ./cmd/glade compat oracle-tests \
  --check docs/fixtures/oracle/fixture-corpus.json \
  --json
```

Focused Salesforce run with recent Apex logs:

```bash
go run ./cmd/glade compat oracle-tests \
  --project example-projects/src-nmb-nutpl-develop \
  --target-org glade-probe-lab \
  --filter SomeTest.someMethod \
  --golden-only \
  --fetch-logs \
  --json
```

Remaining work:

- enable targeted finest logging for selected classes and methods
- add VM trace hooks for final side effects and unsupported fences not already in traces
- promote at least one `src-nmb-nutpl-develop` Salesforce-vs-GLADE passing test into a checked oracle fixture
