# LWC preview

The LWC shell is a local preview surface, not hosted Lightning Experience. It
owns local component routes, preview contexts, and service shims. It does not
own hosted Lightning runtime behavior.

## Use it when

Use LWC preview when you need to open local component, record, app, tab, action,
utility, Flow, or community preview routes with local data and service shims.

## Entry commands

```bash
glade toolchain install
glade dev lwc --project . --open
glade dev lwc --project . --context accountRecord --open
```

## What this module owns

- Workbench Console with Component Lab, Page Workbench, local shell routes,
  preview contexts, and page composer flows.
- Local Apex wire, selected LDS and UI API shims, labels, resources, user data,
  navigation, messages, toast, refresh, bounded Experience managed-content
  reads, and practical base-component shims.
- Toolchain setup for the local preview runtime.

## Requires Salesforce

The LWC shell is a local preview surface, not hosted Lightning Experience.
Salesforce remains the validation gate for hosted platform behavior.

Use Salesforce for exact Lightning Experience chrome, full UI API and GraphQL
semantics, hosted permissions, Experience Builder mutations, learning platform
APIs, live EMP streaming, and every base-component edge.

## Related workflows

- [LWC preview workflow](/guide/workflows/lwc-preview)
- [Local LWC shell](/guide/lwc-local-shell)

## Reference

- [LWC support](/reference/lwc-support)
