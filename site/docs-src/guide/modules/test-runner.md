# Test runner

The test runner owns local Apex test discovery, execution, and result output.
It runs supported tests against Glade's local runtime. It does not own org test
execution in Salesforce.

## Use it when

Use the test runner when you need a fast local Apex test loop, focused changed
tests, watch flows, or CI artifacts from the same runtime contract.

## Entry commands

```bash
glade test --project .
glade test changed --project . --since origin/main
glade test serve --project .
```

## What this module owns

- Local Apex test discovery and execution.
- Test setup, static reset, async drain, isolated local data, and result output.
- Changed-test selection and warm server flows for repeated runs.

## Requires Salesforce

Salesforce remains the validation gate for hosted platform behavior.

Run Salesforce tests for org-only dependencies, live services, installed package
behavior that is not captured locally, and release gates that require an org.

## Related workflows

- [Apex tests](/guide/workflows/apex-tests)
- [Test startup cache](/guide/test-startup-cache)

## Reference

- [CI artifacts](/guide/ci-artifacts)
