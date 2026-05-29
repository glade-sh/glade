---
layout: home

hero:
  name: Glade
  text: Orgless Apex runtime
  tagline: Parse, check, run, test, and serve Salesforce-shaped Apex projects on your own machine.
  actions:
    - theme: brand
      text: Install Glade
      link: /guide/installation
    - theme: alt
      text: Try the Playground
      link: https://play.glade.sh/playground/
    - theme: alt
      text: CLI Reference
      link: /guide/cli-reference

features:
  - title: No org in the inner loop
    details: Glade reads SFDX source from disk and runs supported Apex tests in a local VM.
  - title: One runtime, many surfaces
    details: The same parser, type checker, VM, SOQL, DML, storage, and limits stack backs CLI, LSP, DAP, server, playground, and compatibility checks.
  - title: Salesforce-shaped where it counts
    details: Tests, REST responses, storage fixtures, diagnostics, and compatibility reports use stable machine-readable shapes.
---

## What is Glade?

Glade is a clean-room, open source local Apex runtime. It parses Apex classes and triggers, builds a project index, checks supported semantics, lowers code into an executable representation, and runs it against an in-memory org or an optional SQLite-backed local org.

It is made for the local development loop: check a project, run focused tests, execute anonymous Apex, inspect traces, and exercise a Salesforce-shaped API without pushing source to a scratch org.

::: tip Try it
Open the hosted playground with built-in examples: [play.glade.sh/playground/?example=account-service](https://play.glade.sh/playground/?example=account-service)
:::

## Runtime pipeline

1. Load project configuration and Salesforce metadata.
2. Parse Apex source through the parser adapter.
3. Build symbols and resolve references.
4. Type-check supported semantics.
5. Lower checked code into the VM representation.
6. Execute with SObject, SOQL, DML, triggers, limits, storage, and platform APIs.
7. Surface the same runtime through CLI commands, tests, watch mode, LSP/DAP, profile reports, the local API server, and the playground.

That is the grain of it. One stack. Several handles.
