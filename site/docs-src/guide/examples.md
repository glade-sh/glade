# Built-In Examples

Use `glade examples` to find local playground examples without opening the website.

```bash
glade examples
glade examples --tag limits
glade examples show account-service
glade examples run account-service
```

Open one in the browser workbench:

```bash
glade playground --example account-service --open
```

Useful first examples:

| ID | Use it for |
| --- | --- |
| `account-service` | Classes, SOQL, DML, and tests |
| `deal-desk-discount-guard` | Trigger and limit behavior |
| `limit-counter-drill` | Governor limit counters |
| `org-diff-review-loop` | Local state diffs after DML |

`glade examples run <id>` prints the command that opens the example. It does not modify your project.
