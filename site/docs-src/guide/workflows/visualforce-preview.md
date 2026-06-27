# Preview Visualforce locally

Use the local server for Visualforce controller and page preview work. The local
server gives useful `/apex/<PageName>` previews. It does not promise exact
hosted Visualforce behavior.

## Before you start

Run from the project root. Pick a fixed port for manual browser work or an
ephemeral address with a ready file for scripts.

## Steps

Start the Visualforce preview server:

```bash
glade dev vf --project . --port 8080
```

Start on an available local port and write readiness data:

```bash
glade dev vf --project . --addr 127.0.0.1:0 --ready-file /tmp/glade-vf-ready.json
```

Check the local support endpoint:

```bash
curl http://127.0.0.1:8080/services/data/v61.0/glade/visualforce/support
```

## Expected output

Glade prints the bind address and serves `/apex/<PageName>` routes. The support
endpoint returns local Visualforce capability data for the running server.

## Common wrong turn

Do not treat local preview as exact hosted chrome, lifecycle timing, remoting,
PDF output, or every component edge. Use Salesforce for final hosted
Visualforce checks.

## Deeper reference

- [Visualforce preview](/guide/modules/visualforce-preview)
- [Visualforce support matrix](/reference/visualforce-support)
- [LWC preview](/guide/modules/lwc-preview)
- [What runs locally](/guide/support-map)
