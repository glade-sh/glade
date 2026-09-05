# Built-in examples

Use `glade examples` to find local playground examples without opening the website.

```bash
glade examples
glade examples --tag limits
glade examples show refinement-service
glade examples run refinement-service
```

Open one in the local Playground:

```bash
glade playground --example refinement-service --open
```

Useful first examples:

| ID | Use it for |
| --- | --- |
| `refinement-service` | Classes, SOQL, DML, and tests |
| `deal-desk-discount-guard` | Trigger and limit behavior |
| `limit-counter-drill` | Governor limit counters |
| `org-diff-review-loop` | Local state diffs after DML |

`glade examples run <id>` prints the command that opens the example. It does not modify your project.

The [sample quickstart](/guide/quickstart#try-the-sample) explains how to load
the workspace, run its named test, and inspect a failure. The corrected
`RefinementServiceTest` ships in the v0.2.14 stable release. Never accept Pass
alongside source errors.
