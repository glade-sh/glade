Source-backed Lightning base components live here.

These modules are compiled from `jerry-wang12/lightning-demo` commit
`2f1c6ea4078fd584aea245256073be086d743650`, package license `MIT`.
They are used only for allowlisted base components whose compiled dependency
graph stays local and browser-safe.

Generation command shape:

```bash
node third_party/lwc/compile.mjs < selected-lightning-source-config.json
```

Do not add broad source drops here. Add one component slice at a time, verify
its compiled imports, and keep generated test, perf, docs, and example folders
out of the runtime tree.
