# Debug and profile

Debug and profile tools own local DAP sessions and Apex log analysis. They
read local execution and saved debug logs. They do not own Salesforce replay
debugging or hosted log capture.

## Use it when

Use debug and profile tools when you need DAP stepping, breakpoint checks, Apex
debug-log summaries, or pprof output from local execution logs.

## Entry commands

```bash
glade dap --project .
glade debug profile --log reports/anonymous-output.txt --format markdown
glade profile analyze --log reports/anonymous-output.txt --format pprof
```

## What this module owns

- Debug Adapter Protocol sessions for local Apex.
- Log parsing, trace summaries, and profile output for Apex debug logs.
- Editor and CLI workflows that inspect local execution instead of org replay.

## Requires Salesforce

Salesforce remains the validation gate for hosted platform behavior.

Use Salesforce replay debugging and hosted logs when the run depends on org
state, platform services, or production governor accounting.

## Related workflows

- [Debug Apex workflow](/guide/workflows/debug-apex)
- [Debug Apex in VS Code](/help/debug-apex-vscode)

## Reference

- [Profile Apex debug logs](/help/profile-apex-debug-log)
