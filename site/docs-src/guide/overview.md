# What is Glade?

Glade is a local Apex runtime and developer workbench. It loads Salesforce DX
projects, parses and checks supported Apex, runs local Apex tests, executes
anonymous Apex, serves a Salesforce-shaped local REST API, and exposes support
gaps instead of hiding them.

## First Local Loop

```bash
glade doctor
glade init --project . --yes
glade check --project .
glade test --project . --filter AccountServiceTest
glade test changed --project . --since origin/main
```

## Use Glade When

- You want Apex diagnostics before a deploy.
- You want to run supported Apex tests without logging into an org.
- You want local SOQL, DML, trigger, SObject, and limit feedback.
- You want a Salesforce-shaped local API for development loops.
- You want local assessment, cruft review, or refactor-proof reports for a large Apex project.

## Use Salesforce When

- You need live auth, sessions, identity, or org-hosted process engines.
- You need full Visualforce rendering or PDF generation.
- You need Bulk API, Streaming, Pub/Sub, GraphQL, or broad Tooling API parity.
- You need exact production governor accounting.

## Support Claims

Glade models the local paths it can prove. Unsupported platform services fail
with stable diagnostics instead of pretending to work.

Next: [Quickstart](/guide/quickstart), [Enterprise Workflows](/guide/enterprise-workflows), or [What Glade supports](/guide/support-map).
