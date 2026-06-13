# Compatibility

Glade separates public support claims from maintainer proof reports.

Use the [Support map](/guide/support-map) first. It answers what
works, what works with limits, and what is not supported today.

Use this page when you need the policy behind those claims.

## Status Values

Generated reports track each capability with one of these statuses:

| Status | Meaning |
| --- | --- |
| `supported` | Implemented and covered by compatibility tests. |
| `partial` | Common cases work, with documented gaps. |
| `stub` | Present for loading or controlled placeholders. |
| `unsupported` | Fails with a stable diagnostic before or during execution. |
| `unknown` | Not evaluated yet. |

## Public Layer

The public support map groups the runtime into areas a Salesforce developer can
recognize: Apex front end, local tests, SOQL, DML, SObjects, local API server,
editor tools, standard library, and platform service APIs.

That page favors plain labels: works well, works with limits, not supported, and
not measured.

## Developer Layer

The checked reports live in the repository and are meant for maintainers,
release gates, and compatibility work:

- [`docs/COMPATIBILITY_DASHBOARD.md`](https://github.com/glade-sh/glade/blob/main/docs/COMPATIBILITY_DASHBOARD.md)
- [`docs/STDLIB_COVERAGE.md`](https://github.com/glade-sh/glade/blob/main/docs/STDLIB_COVERAGE.md)
- [`docs/KNOWN_GAPS.md`](https://github.com/glade-sh/glade/blob/main/docs/KNOWN_GAPS.md)

The docs-site summary of those reports lives at
[Maintainer Proof Reports](/guide/compatibility-dashboard).

## How Support Moves

Unsupported behavior moves only when there is runtime behavior and evidence.

1. Add or confirm a compatibility fixture.
2. Implement the smallest runtime behavior that matches the public contract.
3. Run the focused package gate.
4. Regenerate checked compatibility reports when generated rows change.
5. Promote the row from `unsupported` to `partial` or `supported`.

## Report a Gap

When Glade stops at an unsupported surface you need, include:

- the Apex snippet or test that hits the gap
- the command you ran
- the unsupported diagnostic
- whether the behavior is required for local tests, local API use, or editor feedback

## Fixtures

Compatibility fixtures cover parser, semantic, execution, test, storage,
server, and stdlib behavior. They ship through the first-party `compat` plugin,
not as base runtime commands.

## Local-test compatibility

Large local-test gates classify each test as pass, fail, unsupported, load
error, compile error, or internal error. Those gates run through
`glade compat local-tests` after the `compat` plugin is installed.

The clean rule is simple. Add the fixture first. Then change the status.
