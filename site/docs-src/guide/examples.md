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

Create the Refinement Service demo project and exit without starting a server:

```bash
glade playground --data-root .glade/playground --db .glade/playground/org.sqlite --example refinement-service --once
```

The project is written to `.glade/playground/workspaces/default`. The example
flag refuses a non-empty managed workspace unless you explicitly pass the
destructive `--reset-on-start` flag, so run it from a fresh evaluation
directory. Then initialize and run its named test:

```bash
GLADE_PROJECT=.glade/playground/workspaces/default
test -f "$GLADE_PROJECT/glade.yml" || glade init --project "$GLADE_PROJECT" --yes
glade doctor --project "$GLADE_PROJECT"
glade test --project "$GLADE_PROJECT" --class RefinementServiceTest --method createsAndLabelsFileRow --json --no-progress
```

Open the same project in the browser workbench when you want to explore it:

```bash
glade playground --project "$GLADE_PROJECT" --db .glade/playground/org.sqlite --open
```

Useful first examples:

| ID | Use it for |
| --- | --- |
| `refinement-service` | Classes, SOQL, DML, and one named test |
| `deal-desk-discount-guard` | Trigger and limit behavior |
| `limit-counter-drill` | Governor limit counters |
| `org-diff-review-loop` | Local state diffs after DML |

`glade examples run <id>` prints the browser command. It does not modify your
Salesforce DX project. `glade playground --example <id>` writes only to its
managed scratch workspace. Use the [five-minute Quickstart](/guide/quickstart#sample-project)
for the checked first-run sequence and cleanup.

The bundled `RefinementServiceTest` first shipped in v0.2.14 and is included in
the v0.2.15 stable release.
