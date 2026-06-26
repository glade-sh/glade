# glade-tools

`glade-tools` is the first-party maintainer toolkit. It feeds selected
maintainer docs into this site, but the product repository does not depend on
tool internals.

Use it for compatibility fixtures, capability catalogs, dashboards, stdlib
ledgers, support scanners, surface-ledger refresh, open-source corpus scans,
Salesforce docs inventories, catalog reconcile work, and plugin archives.

Use product Glade for runtime behavior and user workflows. Use glade-tools when
the job is to inspect, compare, capture, or package maintenance material.

`@glade/compat` keeps its package name for this release. A later release may
introduce `@glade/maintainer` as a friendlier name after the registry and
release train are stable.

## Proof

Run these in the sibling tools checkout:

```bash
go test ./...
go run ./cmd/glade-plugin-compat manifest --json
scripts/build-plugin-archives.sh 0.2.0
```

Repo-only notes and generated tool reports stay in the tools checkout.
