---
pageType: recovery
canonicalTask: /help/troubleshooting
title: Troubleshoot Glade
description: Recover from common Glade project discovery, doctor, test, VS Code, local target, and plugin setup problems.
---

# Troubleshoot Glade

<div class="docs-intro">
  <p class="docs-intro-eyebrow">Troubleshooting</p>
  <p>Start with the symptom you recognize. Confirm project context before changing Apex source or local data.</p>
</div>

## Glade cannot find my project

Confirm the shell is inside the Salesforce DX project, then initialize and
recheck the project-aware environment:

```bash
test -f sfdx-project.json
test -f glade.yml || glade init --project . --yes
glade doctor --project .
```

If the first command fails, move to the directory that owns
`sfdx-project.json`. Continue with the [first local check](/guide/quickstart).

## `glade doctor` fails

Read the first failed status row. A project failure means the working directory
or `--project` path is wrong. A parser or toolchain failure is an installation
problem. Re-run `glade version`, then follow [Installation](/guide/installation).

## A test is not discovered

Run the project suite without a selector, then select a class found in its
results. This command executes tests; it is not an inventory-only operation:

```bash
glade test --project .
glade test --project . --class <YourTestClass>
```

Use a class that exists in the active package directories. See [Run one Apex
test](/help/run-one-apex-test).

## Local and Salesforce results differ

Record the exact Glade command and result, then check the capability in [What
runs locally](/guide/support-map). Hosted services and exact production
behavior remain Salesforce validation. Report a reproducible mismatch with the
smallest source, metadata, and data fixture that shows it.

## VS Code cannot find the Glade binary

Run `glade editor doctor vscode` in a terminal where `glade version` succeeds.
Restart VS Code after changing `PATH`. See [Use Glade in VS Code](/guide/editor).

## A breakpoint is not hit

First prove the selected test runs locally. Then confirm the launch points at
the same project and class. Follow [Debug Apex with
breakpoints](/help/debug-apex-vscode).

## A local `sf` target is missing

Start and authorize the named local target again. The isolated-shell and
cleanup rules are in [Set up a Glade org and import data with
`sf`](/help/glade-org-sf-data-import).

## A plugin is not restored in CI

Commit the plugin lock file, use the same Glade version locally and in CI, and
follow [Plugin lock files and CI](/guide/plugins/lock-ci). Plugins are separate
executables; a base Glade install does not restore them implicitly.

## A test command exits zero but runs no tests

Read the JSON `summary.total` and the selected method names. Confirm the class
and method exist in the active package directories. An empty changed-test
selection can be legitimate; run a known relevant class or suite to establish
execution evidence. Use the [first-run sample](/guide/quickstart#sample-project)
when diagnosing installation separately from project behavior.

## Verify recovery

Repeat the exact command that failed in the same project and local environment.
Check its diagnostic and executed-test counts as well as its exit code.

## Still blocked

Include `glade version`, `glade doctor --project .`, the exact command, stable
error code, and the smallest reproducible project state in a [GitHub
issue](https://github.com/glade-sh/glade/issues). Remove credentials, private
source, and customer records first. Vulnerabilities use
[private reporting](/guide/security-trust#report-a-vulnerability).
