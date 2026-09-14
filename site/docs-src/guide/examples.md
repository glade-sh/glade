---
pageType: guide
canonicalTask: /guide/examples
---

# Built-in examples

Use `glade examples` to find local playground examples without opening the website.

```bash
glade examples
glade examples --tag limits
glade examples show refinement-service
glade examples run refinement-service
```

Open the Refinement Service example in the local browser workbench:

```bash
GLADE_EXAMPLE_DIR="$(mktemp -d)"
glade playground --data-root "$GLADE_EXAMPLE_DIR/playground" --db "$GLADE_EXAMPLE_DIR/org.sqlite" --example refinement-service --open
```

Stop the workbench with Ctrl-C after its files load. From the same directory,
initialize and run the named test against its managed workspace:

```bash
GLADE_PROJECT="$GLADE_EXAMPLE_DIR/playground/workspaces/default"
test -f "$GLADE_PROJECT/glade.yml" || glade init --project "$GLADE_PROJECT" --yes
glade doctor --project "$GLADE_PROJECT"
glade test --project "$GLADE_PROJECT" --class RefinementServiceTest --method createsAndLabelsFileRow --json --no-progress
```

Useful first examples:

| ID | Use it for |
| --- | --- |
| `refinement-service` | Classes, SOQL, DML, and one named test |
| `deal-desk-discount-guard` | Trigger and limit behavior |
| `limit-counter-drill` | Governor limit counters |
| `org-diff-review-loop` | Local state diffs after DML |

`glade examples run <id>` prints the browser command. It does not modify your
Salesforce DX project. The browser command writes the example to its managed
scratch workspace and may create the configured SQLite org database beside it.
`--reset-on-start` clears both managed workspace and org state. Use the
[five-minute Quickstart](/guide/quickstart#sample-project) for a terminal-only
first run and cleanup. The command above uses a fresh data root because the
v0.2.15 stable binary predates the non-empty-workspace guard.

The bundled `RefinementServiceTest` first shipped in v0.2.14 and is included in
the v0.2.15 stable release.
