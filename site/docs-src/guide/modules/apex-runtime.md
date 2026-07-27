# Apex runtime

The Apex runtime is the local parser, semantic analyzer, VM, SOQL, DML,
trigger, and schema surface. It owns supported local execution. It does not own
hosted Salesforce services.

## Use it when

Use the Apex runtime when you need local parse, semantic checks, symbol
inspection, or anonymous Apex before a Salesforce validation pass.

## Entry commands

```bash
glade check --project .
glade exec --project . "System.debug('local');"
glade inspect symbols --project .
```

## What this module owns

- Apex parsing, project loading, symbols, and semantic diagnostics.
- Local VM execution for supported Apex, SOQL, DML, triggers, and SObjects.
- Unsupported diagnostics for hosted-only platform behavior.

## Requires Salesforce

Salesforce remains the validation gate for hosted platform behavior.

Use Salesforce for live org services, exact governor accounting, hosted
metadata behavior, and any platform feature Glade reports as unsupported.

## Related workflows

- [Enterprise workflows](/guide/enterprise-workflows)
- [Support map](/guide/support-map)

## Reference

- [Apex language compatibility](/reference/apex-language-compatibility)
- [Apex support](/reference/apex-support)
