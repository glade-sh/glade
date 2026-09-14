---
pageType: recovery
canonicalTask: /help/troubleshooting
title: Troubleshoot Glade
description: Diagnose project discovery, installation, test selection, and editor problems. Follow scoped recovery steps and report a sanitized reproduction.
---

# Troubleshoot Glade

<div class="docs-intro">
  <p class="docs-intro-eyebrow">Troubleshooting</p>
  <p>Start with the symptom you recognize. Confirm project context before changing Apex source or local data.</p>
</div>

## Glade cannot find my project

Run this from the directory that contains `sfdx-project.json`. The block stops
before initialization when the project file is missing.

```bash
(
  test -f sfdx-project.json || {
    printf '%s\n' 'No sfdx-project.json here. Move to your Salesforce DX project root, then retry.' >&2
    exit 1
  }
  if test ! -f glade.yml; then
    glade init --project . --yes || exit 1
  fi
  glade doctor --project . || exit 1
)
```

Initialization can create `glade.yml`. Inspect that file before committing it.
A failure here does not establish that your Apex source is invalid. Continue
with the [first local check](/guide/quickstart).

## `glade doctor` fails

Read the first failed status row. A project failure means the working directory
or `--project` path is wrong. A parser or toolchain failure is an installation
problem. Re-run `glade version`, then follow [Installation](/guide/installation).

## A test is not discovered

Find a known test class in the active package directories listed by your
project's configuration. Read its class declaration and test methods; do not
use an entire suite merely as an inventory command.

From the project root, enter that class name when prompted:

```bash
(
  test -f sfdx-project.json || {
    printf '%s\n' 'Move to the Salesforce DX project root first.' >&2
    exit 1
  }
  printf 'Known test class name: '
  IFS= read -r test_class || exit 1
  case "$test_class" in
    ''|*[!A-Za-z0-9_.]*)
      printf '%s\n' 'Enter a nonempty class name, without spaces or angle brackets.' >&2
      exit 1 ;;
  esac
  glade test --project . --class "$test_class"
)
```

This command **executes the selected class**. Check the executed-test count,
not just the exit status. When you need to separate installation from your
project's behavior, use the [small first-run
sample](/guide/quickstart#sample-project).

For a bug report, record the exact selected name, Glade version, expected test
count, and actual count. Share only source and diagnostics you are authorized
to publish. Security reports use the existing [private
route](/guide/security-trust#report-a-vulnerability).

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
