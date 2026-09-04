# Preview LWC locally

Use the local shell for LWC routes, page targets, named contexts, and record
data. The local shell gives useful local preview routes. It does not replace
hosted Lightning Experience.

## Before you start

Packaged releases include the local toolchain. Run from the project root so Glade can find
source, `glade.lwc.json`, schema, labels, static resources, and local data.

## Steps

Verify the bundled LWC toolchain:

```bash
glade toolchain status
```

Expected: the installed toolchain is available. A release user does not need
a Glade source checkout. If it is missing, repair the packaged installation;
source-copy installation belongs to the source-development workflow.

Each LWC bundle must declare an exact supported API version (65.0, 66.0, or
67.0). Source, bundle, and HTTP endpoint versions are independent; do not
upgrade metadata merely to hide an unsupported-version result.

Open the LWC Workbench Console:

```bash
glade dev lwc --project . --open
```

Use a seeded local DB when record-page LWCs, LDS, Apex, or builder record
search need persisted rows:

```bash
glade db seed --db .glade/envs/lwc-preview.sqlite --project . data/lwc-preview-db.json
glade dev lwc --project . --db .glade/envs/lwc-preview.sqlite --open
```

Open a named context:

```bash
glade dev lwc --project . --context accountRecord --open
```

Open an explicit record page target:

```bash
glade dev lwc --project . --target record-page --object Account --record 001000000000001AAA --page Account_Record_Page --open
```

## Expected output

Glade prints a local URL and opens the Workbench Console when `--open` is set.
Component Lab filters exposed LWCs, reads target metadata and `@api`
properties, edits page context and preview properties, and switches form
factors. Page Workbench groups discovered routes. The debug dock records
Console, Apex, LDS Cache, Network, Events, Issues, and recent save or rebuild
runs. Builder record pages search the active local DB, while app and home page
contexts stay record-free. Startup prints at most eight routes; use a ready file
or `/lightning/local/context.json` for the complete list.

## Common wrong turn

Do not use the local shell as final proof for Lightning Experience chrome,
hosted permission behavior, every base component edge, Flow Builder behavior, or
live streaming. Salesforce remains the gate for hosted behavior.

## Deeper reference

- [LWC preview](/guide/modules#lwc-preview)
- [LWC support matrix](/reference/lwc-support)
- [Local org and data](/guide/modules#local-org-and-data)
- [Local LWC shell details](/guide/lwc-local-shell)
