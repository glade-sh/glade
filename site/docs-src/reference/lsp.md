# LSP reference

<div class="docs-intro">
  <p class="docs-intro-eyebrow">Reference</p>
  <p>Invoke Glade language services, enable the optional VS Code client, and use the project graph from terminal tooling.</p>
</div>

Verified against the stable Glade release shown in the site header.

## Invocation

Start the Language Server Protocol process over stdio:

```bash
glade lsp --project .
```

Run one diagnostics pass without a long-lived client:

```bash
glade lsp --project . --diagnostics-once
```

## VS Code configuration

The bundled extension keeps the Glade LSP off by default so it does not take
over Salesforce language-server ownership. Set `glade.enableLsp=true` when local
Glade diagnostics are wanted in the workspace.

## Related code-intelligence commands

```bash
glade inspect definition --project . --symbol RefinementService
glade inspect definition --project . --file force-app/main/default/classes/RefinementService.cls --line 6 --column 13
glade inspect references --project . --symbol RefinementService.total --json
glade inspect references --project . --symbol Account.Name --include-declaration
glade refactor rename --project . --symbol RefinementService --to FileRefinementService --dry-run
glade schema import describe --input reports/org-describe.json --project-cache .
```

`glade refactor rename` defaults to a dry-run plan. `schema import describe
--project-cache` writes captured schema symbols under `.glade/symbols`.

## Output and troubleshooting

The server writes protocol messages to stdio. Run `--diagnostics-once` first
when checking project discovery, parser availability, or diagnostic content.
Use `glade doctor --project .` for setup failures and the [error-code
reference](/reference/errors) for stable diagnostics.

## Boundary

The LSP reports supported local project analysis. Live org symbols, deployment,
and Salesforce-hosted language behavior remain Salesforce validation work.
