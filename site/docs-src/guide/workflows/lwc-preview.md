# Preview LWC locally

Use the local shell for LWC routes, page targets, named contexts, and record
data. The local shell gives useful local preview routes. It does not replace
hosted Lightning Experience.

## Before you start

Install the local toolchain once. Run from the project root so Glade can find
source, `glade.lwc.json`, schema, labels, static resources, and local data.

## Steps

Install the LWC toolchain:

```bash
glade toolchain install
```

Open the Workbench Console:

```bash
glade dev lwc --project . --open
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
The console shows route discovery, a preview canvas, editable context, debug
panes for Apex, LDS, network calls, navigation/events, and runtime issues, and
local data service behavior. Builder mobile preview uses the main canvas
viewport control instead of a permanent side-by-side phone panel.

## Common wrong turn

Do not use the local shell as final proof for Lightning Experience chrome,
hosted permission behavior, every base component edge, Flow Builder behavior, or
live streaming. Salesforce remains the gate for hosted behavior.

## Deeper reference

- [LWC preview](/guide/modules/lwc-preview)
- [LWC support matrix](/reference/lwc-support)
- [Local org and data](/guide/modules/local-org-data)
- [Local LWC shell details](/guide/lwc-local-shell)
