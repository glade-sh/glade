# Visualforce preview

The Visualforce server is a local preview surface, not hosted Visualforce. It
owns local page routes, controller flows, and preview diagnostics. It does not
own exact hosted Visualforce rendering.

## Use it when

Use Visualforce preview when you need local controller work, page routes,
common component rendering, remoting envelopes, uploads, or preview support
inspection.

## Entry commands

```bash
glade dev vf --project . --port 8080
curl http://127.0.0.1:8080/services/data/v61.0/glade/visualforce/support
```

## What this module owns

- Local `/apex/<PageName>` routes and Visualforce page rendering.
- Controller helpers, PageReference paths, messages, static resources, signed
  view state, CSRF checks, remoting, AJAX refresh, and local PDF fallback output.
- Diagnostics for Lightning Out and LWC dependencies used from Visualforce.

## Requires Salesforce

The Visualforce server is a local preview surface, not hosted Visualforce.
Salesforce remains the validation gate for hosted platform behavior.

Use Salesforce for hosted chrome, exact lifecycle timing, every component edge,
`PageReference.getContent*`, and byte-for-byte PDF output.

## Related workflows

- [Visualforce preview workflow](/guide/workflows/visualforce-preview)
- [Support map](/guide/support-map)

## Reference

- [Visualforce support](/reference/visualforce-support)
