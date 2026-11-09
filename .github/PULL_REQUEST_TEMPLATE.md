## Ticket and change class

Write `n/a: <reason>` in any field that does not apply.

Ticket: <id>. Change class (one): docs | tests | tooling | behavior | performance | generated.
See [docs/WORKFLOW.md](../docs/WORKFLOW.md) section 4 for what each class needs.

## What changes for a user

<one to three sentences, or "none" for an internal change>

## Evidence (behavior changes)

```
NATIVE <behavior> <path>/org.tsv:<line> <route> <api>
```

One line per behavior change. Fail-before: test commit `<sha>` fails on its own; fix commit `<sha>`.
Route twins: <routes covered, or `n/a: <reason>`>.

## Checks run

| Check | Command | Result |
| --- | --- | --- |
| Focused tests | `CGO_ENABLED=1 go test ./internal/<pkg> -run '...'` | |
| Package tests, once (apextest: focused `-run` tests; CI runs 12 shards) | `CGO_ENABLED=1 go test ./internal/<pkg>` | |
| Repository guard | `CGO_ENABLED=1 go test ./internal/repoguard` | |
| Format and vet | `gofmt -l .` and `go vet ./internal/<pkg>` | |

List every check not run and why. Zero selected tests are not a pass. For a
visible UI change, add sanitized desktop and mobile screenshots.

## Risk and revert

<what could break; how to revert, normally by reverting the squash commit>

## Checklist

- [ ] No third-party project names, paths or source; no credentials, org ids, aliases or personal paths.
- [ ] Generated files changed through their generator or input, and both committed.
- [ ] Every edit or deletion of an existing test cites a native row.
- [ ] ABI constant bumped when the change class requires it.
- [ ] New package or new `t.Parallel` test: `scripts/ci-package-lanes.json` and the inventory tests updated.
- [ ] User-visible change recorded in `docs/RELEASE_NOTES.md` under Unreleased.
- [ ] The diff touches only the owned files; unrelated work is preserved.

Security-sensitive details belong in the project's private reporting channel, not in this public pull request.
