---
name: browser-suites
description: Use when a change touches the LWC runtime, the LWC shell, Visualforce browser conformance (V09, V12, V13, V15) or the L08 server conformance test, or when a test skips for Node, Playwright or Chromium.
---

# Browser suites

A skipped browser test is not a pass. A passing ordinary `go test` run does not
establish L08 or Visualforce browser parity.

## Install

    npm ci --prefix third_party/lwc
    npm ci --prefix lwcruntime
    # Chromium, with the Playwright version pinned in lwcruntime
    ./lwcruntime/node_modules/.bin/playwright install chromium

On Linux add `--with-deps`. Check the prepared compiler without changing it:

    node third_party/lwc/scripts/prepare-lwc-shared.mjs --check <third_party/lwc dir>

## Run

LWC runtime and generated base components:

    GLADE_LWC_BROWSER=1 CGO_ENABLED=1 go test -p=1 -count=1 -timeout=25m \
      -run '^(TestBrowserRuntimeSuite|TestGeneratedPhase3BaseComponentsRunInBrowser)$' \
      ./internal/lwcruntime ./internal/lwcbrowser

L08 server conformance (an enabled run fails at once when a prerequisite is
missing):

    GLADE_HOME="$PWD" GLADE_LWC_BROWSER=1 CGO_ENABLED=1 go test -p=1 -count=1 -timeout=0 \
      -run '^TestL08SalesforceConformance$' ./internal/server

Visualforce conformance:

    CGO_ENABLED=1 go test -count=1 -p=1 -timeout=0 \
      -run '^TestV(09|12|13|15).*SalesforceConformance$' ./internal/visualforce

## Environment

- Archived checkout without `node_modules`: set `GLADE_LWC_PLAYWRIGHT_MODULE`
  to the absolute Playwright package directory and `GLADE_HOME` to a prepared
  toolchain. `GLADE_LWC_BROWSER_EXECUTABLE` selects an installed Chromium.
- LWC family conformance runs one API at a time. `GLADE_LWC_BROWSER_API_PARALLEL=2`
  with `-parallel=2` runs two, and only on a host with at least 16 GiB of memory.
- Site browser tests: use `CI=1` so Playwright does not reuse a server on port
  4173. Do not stop another task's server.

## Reporting

Say which suites ran, which were skipped and why. Without Node, Playwright or
Chromium, report "not run: <tool>" and leave the result to hosted CI.
Reference: `docs/AI_SETUP.md`.
