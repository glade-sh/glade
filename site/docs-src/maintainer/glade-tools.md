# glade-tools

`glade-tools` is the first-party maintainer toolkit. It feeds selected
maintainer docs into this site, but the product repository does not depend on
tool internals.

Use it for compatibility fixtures, capability catalogs, dashboards, stdlib
ledgers, support scanners, surface-ledger refresh, open-source corpus scans,
Salesforce docs inventories, catalog reconcile work, and plugin archives.

Use product Glade for runtime behavior and user workflows. Use glade-tools when
the job is to inspect, compare, capture, or package maintenance material.

`@glade/compat` is the published maintainer package. Keep it out of first-run
user setup.

## Proof

Run these in the sibling tools checkout:

```bash
go test ./...
go run ./cmd/glade-plugin-compat manifest --json
scripts/build-plugin-archives.sh X.Y.Z
```

Repo-only notes and generated tool reports stay in the tools checkout.

## Local-test comparison

Use `glade compat local-tests compare` only with an external target manifest.
The required flags are `--base-bin`, `--candidate-bin`, `--project`, `--out`,
`--workers`, `--runs 5`, and `--manifest`:

```bash
glade compat local-tests compare \
  --base-bin /path/to/base/glade-plugin-compat \
  --candidate-bin /path/to/candidate/glade-plugin-compat \
  --project /path/to/project \
  --out /private/tmp/glade-local-tests-X.Y.Z \
  --workers 1 \
  --runs 5 \
  --manifest /path/to/targets.json \
  --json
```

`--out` must name a new private directory outside the source project. Each
target gets five cold alternating AB/BA pairs in exactly `AB, BA, AB, BA, AB`
order. The command writes a deterministic `summary.json` plus the referenced
raw run evidence. Requested profiles run afterward for diagnostics and are
excluded from timing samples.

## Standard-describe pack generation

Generate describe packs from one JSON or gzip-compressed JSON source with all
four explicit paths. The output parents must already exist:

```bash
(cd ../glade && node ../glade-tools/scripts/generate-standard-describe-pack.mjs \
  internal/storage/standard_describe_catalog.json.gz \
  internal/storage/standard_describe_catalog_v2.pack \
  internal/storage/standard_describe_child_relationships_v2.pack \
  internal/storage/standard_describe_catalog_v2_index_generated.go)
```

The generator writes canonical catalog and reverse packs plus the Go index;
never hand-edit those outputs. It rejects input/output aliases and publishes
through sibling temporary files with rollback protection, so a failed or
interrupted publish restores the prior complete output set.

## Plugin release

The first-party packages are `@glade/compat`, `@glade/performance`, and
`@glade/orgpackage`. GitHub plugin release metadata and assets are immutable
on rerun. A corrected archive or release note requires a new version, not an
overwrite of the published release or its versioned registry object.
