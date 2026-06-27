# Product areas

Use these pages to choose the Glade surface for a local job. Start with the
command in the table, then open the area page for boundaries and related flows.

| Area | Use it for | Start with |
| --- | --- | --- |
| [Apex runtime](/guide/modules/apex-runtime) | Check Apex, inspect symbols, and run anonymous Apex against the local runtime. | `glade check --project .` |
| [Test runner](/guide/modules/test-runner) | Run local Apex tests, changed-test loops, and warm test servers. | `glade test --project .` |
| [Local org and data](/guide/modules/local-org-data) | Create a named local org, attach project auth, and serve local data APIs. | `glade org create refinement-local` |
| [LWC preview](/guide/modules/lwc-preview) | Open local Lightning Web Component preview routes and data contexts. | `glade dev lwc --project . --open` |
| [Visualforce preview](/guide/modules/visualforce-preview) | Serve local Visualforce pages and inspect preview support. | `glade dev vf --project . --port 8080` |
| [Debug and profile](/guide/modules/debug-profile) | Start DAP debugging and analyze Apex debug logs. | `glade dap --project .` |
| [Editor and workbench](/guide/modules/editor) | Install or check the VS Code extension and run local diagnostics. | `glade editor doctor vscode` |
| [Plugins](/guide/modules/plugins) | List, discover, and link extension executables outside the base product. | `glade plugins list` |
