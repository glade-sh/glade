# Config reference

This page is a stable reference entry point. Use the linked detailed page or generated artifact when you need the full table.

Use this page when you need the project configuration trail.
The detailed guide carries the current keys and examples.

## Start here

Start with `glade init --project . --yes`.
Then inspect what Glade reads with `glade config show --project .`.
Run `glade config validate --project .` before sharing a config change.

`glade.yml` carries Glade runtime choices.
`sfdx-project.json` remains the Salesforce project file.
Glade layers both, with CLI flags on top.

## Detailed source

[Configure a Glade project](/guide/configuration)

## Related workflows

- [Local org data](/guide/modules/local-org-data)
- [Plugins](/guide/modules/plugins)
- [Error codes](/reference/errors)
