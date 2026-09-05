---
pageType: hub
canonicalTask: /help/
title: Glade Help
description: Complete a focused Glade task or recover from a recognizable project, test, editor, data, or CI problem.
---

# Glade Help {#task-guides-and-troubleshooting}

## Fix a problem

Start with the symptom. Diagnose the same project and selected local environment
before changing source or resetting data.

| What happened | First safe check | Continue with |
| --- | --- | --- |
| Command not found | `glade version` and the install directory on `PATH` | [Installation](/guide/installation) |
| Project or doctor failure | `test -f sfdx-project.json`, then read the first failed doctor row | [Project recovery](/help/troubleshooting#glade-cannot-find-my-project) |
| No test result or zero tests | Check the JSON total and actual class/method names | [Test discovery](/help/troubleshooting#a-test-is-not-discovered) |
| A breakpoint never stops | Run the selected test without the debugger | [Breakpoint recovery](/help/troubleshooting#a-breakpoint-is-not-hit) |
| A local target is missing | Check the same project, alias, and isolated sf configuration | [Local target setup](/help/glade-org-sf-data-import) |
| CI loses results | Preserve nonzero exits and upload existing artifacts after failures | [CI setup](/help/ci-setup) |
| Local results differ from Salesforce | Name the capability, local result, and hosted boundary | [Compatibility](/guide/support-map) |

[Open all troubleshooting paths](/help/troubleshooting)

## Complete a task

Use [Guides](/guide/workflows) for the main task paths. These illustrated
walkthroughs retain interface-specific steps and recovery details.

<div class="docs-route-grid">
  <a class="docs-route-card" href="/help/first-local-check"><strong>Run the first local check</strong><span>Outcome: a configured project and a checked source tree.<br>Interface: terminal.</span></a>
  <a class="docs-route-card" href="/help/run-one-apex-test"><strong>Run one Apex test</strong><span>Outcome: focus a local test loop on one class or method.<br>Interface: terminal and VS Code.</span></a>
  <a class="docs-route-card" href="/help/debug-apex-vscode"><strong>Debug Apex with breakpoints</strong><span>Outcome: stop in a local Apex test and inspect state.<br>Interface: VS Code.</span></a>
  <a class="docs-route-card" href="/help/anonymous-apex-scratch"><strong>Use anonymous Apex scratch</strong><span>Outcome: execute a local snippet and inspect output.<br>Interface: VS Code.</span></a>
  <a class="docs-route-card" href="/help/local-data-environments"><strong>Work with local data</strong><span>Outcome: seed and query a local environment.<br>Interface: terminal.</span></a>
  <a class="docs-route-card" href="/help/changed-tests-before-pr"><strong>Run changed tests before a PR</strong><span>Outcome: select only tests affected by your changes.<br>Interface: terminal and CI.</span></a>
  <a class="docs-route-card" href="/help/profile-apex-debug-log"><strong>Profile an Apex debug log</strong><span>Outcome: analyze saved debug-log events locally.<br>Interface: terminal.</span></a>
  <a class="docs-route-card" href="/help/ci-setup"><strong>Add Glade to CI</strong><span>Outcome: emit a reliable local check, test, and report gate.<br>Interface: CI.</span></a>
</div>

## Check support and trust

[Report a bug or share workflow feedback](https://github.com/glade-sh/glade/issues/new/choose).
Include the command, version, OS, expected/actual result, and a minimal public
reproduction. Do not upload proprietary source, private package names,
credentials, or customer records. Report vulnerabilities through the
[private security route](/guide/security-trust), not public issues.

Confirm what Glade runs locally and review the release and plugin trust boundaries.

<div class="docs-route-grid">
  <a class="docs-route-card" href="/guide/support-map"><strong>What Glade runs locally</strong><span>Search supported APIs, named limits, and paths that still require Salesforce.</span></a>
  <a class="docs-route-card" href="/guide/security-trust"><strong>Security & trust</strong><span>Verify releases, checksums, SBOMs, attestations, and plugin execution boundaries.</span></a>
</div>

Use the [error reference](/reference/errors) when output includes a stable Glade
error code.
