# Oracle Parity

`oaer compat oracle-tests` compares Salesforce observations with OAER observations.
The pass/fail input is normalized JSON, not raw debug log text.

Current skeleton:

- normalizes IDs, timestamps, generated usernames, stack line noise, records, side effects, and trace events
- parses Apex logs for method calls, SOQL, DML, exceptions, limits, `USER_DEBUG`, and `OAER_ORACLE:` payloads
- adapts OAER `testreport.Run` results into `OracleRun`
- diffs fixture or live runner output and writes compact artifacts under `.oaer/runs/<run-id>/oracle/`

Fixture diff:

```bash
go run ./cmd/oaer compat oracle-tests \
  --salesforce-run docs/fixtures/oracle/passing-test-salesforce.json \
  --local-run docs/fixtures/oracle/passing-test-local.json \
  --json
```

Forced trace mismatch:

```bash
go run ./cmd/oaer compat oracle-tests \
  --salesforce-run docs/fixtures/oracle/passing-test-salesforce.json \
  --local-run docs/fixtures/oracle/trace-mismatch-local.json \
  --json
```

Checked fixture corpus:

```bash
go run ./cmd/oaer compat oracle-tests \
  --check docs/fixtures/oracle/fixture-corpus.json \
  --json
```

Remaining work:

- fetch Apex logs and Tooling records for full Salesforce test runs
- enable targeted finest logging for selected classes and methods
- add VM trace hooks for final side effects and unsupported fences not already in traces
- promote at least one `src-nmb-nutpl-develop` Salesforce-vs-OAER passing test into a checked oracle fixture
