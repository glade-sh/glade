# Choose a Glade workflow

Start with the job in front of you. Each workflow gives the commands, the local
result to expect, and the Salesforce boundary to keep.

## Start here

| Workflow | Use it when |
| --- | --- |
| [Run Apex tests](/guide/workflows/apex-tests) | You want a local Apex test loop for all tests, one class, one method, changed tests, or failed tests. |
| [Debug Apex](/guide/workflows/debug-apex) | You need breakpoints, anonymous Apex output, or a profile from a local debug log. |
| [Preview LWC locally](/guide/workflows/lwc-preview) | You want a local shell for LWC routes, records, page targets, and named contexts. |
| [Preview Visualforce locally](/guide/workflows/visualforce-preview) | You want local `/apex/<PageName>` preview routes for controller and page work. |
| [Work with local data](/guide/workflows/local-data) | You need a local sf target, queryable records, or a seeded SQLite store. |
| [Add Glade to CI](/guide/workflows/ci) | You need SARIF, JSON, JUnit, and stable exit codes in pull request checks. |

## When you need more depth

Use [Product areas](/guide/modules) when you need the system map behind a
workflow. Use the [CLI reference](/reference/cli) when you need exact flags. Use
[What runs locally](/guide/support-map) before relying on local behavior for a
hosted Salesforce edge.
