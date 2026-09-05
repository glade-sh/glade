---
pageType: guide
canonicalTask: /guide/tester-field-guide
---

# Pilot Glade on a real project

<div class="docs-intro">
  <p class="docs-intro-eyebrow">Adoption guide</p>
  <p>Compare one representative local workflow with Salesforce before making Glade a team or CI gate.</p>
</div>

Glade does not replace Salesforce. This pilot establishes where a faster local
loop is useful and where hosted validation remains required.

The git examples assume `origin/main` is your intended base ref and is available
locally. Substitute the correct existing ref for your repository before running
changed-test or refactor commands.

## Before you start

Choose a Salesforce DX project you are authorized to evaluate and a
representative local task. The installation and initialization steps are
included below; Salesforce comparison needs a separately authorized org.

## 1. Choose a representative path

Pick one project and one frequent job: check Apex, run one test class, debug a
test, or run changed tests. Avoid starting with the largest suite or a path
dominated by hosted services.

Record:

- the Salesforce DX project and package directory
- the exact Salesforce command/result used as the comparison
- the Glade version under evaluation
- whether the path uses live auth, hosted services, or org-only metadata

## 2. Establish project context

```bash
curl -fsSL https://glade.sh/install.sh | sh
export PATH="$HOME/.local/bin:$PATH"
glade version
cd path/to/salesforce-dx-project
test -f glade.yml || glade init --project . --yes
glade config validate --project .
glade doctor --project .
```

Expected: doctor identifies the project and ends with `Ready.` before the pilot
uses source checks or tests.

## 3. Run the local equivalent

```bash
glade check --project .
glade test --project . --class RefinementServiceTest --json
```

Use an actual class from the project. Save the command, exit code, diagnostic or
test result, runtime, and any local files written.

## 4. Compare the evidence

Compare result semantics, not only whether both commands passed. Record:

- diagnostics and source locations
- selected tests and pass/fail outcomes
- changed local data or fixtures
- named unsupported behavior
- the matching Salesforce result

Use [What Glade runs locally](/guide/support-map) to classify a difference as a
supported local path, a named limit, or behavior that requires Salesforce.

## 5. Try the daily interface

For VS Code, install the bundled extension and open Glade Home:

```bash
glade editor doctor vscode
glade editor install vscode --force
```

Confirm one diagnostic navigation, Test Explorer or CodeLens run, and—if it is
part of the chosen path—one debug session or Apex & SOQL scratch buffer.

## 6. Add a nonblocking CI trial

Use the pinned [advisory pilot workflow](/guide/ci-artifacts#advisory-pilot).
Its assessment and test steps use `continue-on-error: true`; setup failures
remain failures, and artifacts upload with `if: always()`. Full git history
is needed for affected-test selection. Do not make this job a required merge
gate during evaluation.

Keep the trial advisory until representative local results have been compared
with Salesforce. Promote only the paths the team has reviewed.

## 7. Exercise one deeper workflow

Choose only what the pilot needs:

- [AI-assisted Apex](/guide/ai-assisted-apex) for a repeatable agent check/fix/rerun contract
- `glade report refactor-proof --project . --since origin/main` for change evidence
- `glade dev vf --project . --addr 127.0.0.1:8080` for supported local Visualforce preview
- `glade dev lwc --project . --open` for the LWC Workbench Console
- [Plugins](/guide/plugins) when the default public plugin registry serves a needed first-party package

Exact hosted Visualforce, Lightning Experience, live auth, deployment, and
production governor behavior remain Salesforce validation work.

## 8. Decide and report

Useful pilot feedback includes:

- `glade version`
- full `glade doctor --project .` output
- the exact command and exit code
- the smallest source, metadata, fixture, or saved debug log that shows a mismatch
- the Salesforce comparison result
- the support-map classification

Open a reproducible [GitHub issue](https://github.com/glade-sh/glade/issues) for
an unexplained supported-path mismatch. Use neutral project labels. Do not
include proprietary source, private package names, credentials, customer
records, or unredacted support bundles. Vulnerabilities belong in
[private security reporting](/guide/security-trust), not public issues.

Record a small manual table; no new telemetry is required:

| Evaluation | Record |
| --- | --- |
| Environment | Neutral project label, OS/architecture, product/plugin versions, source API version |
| First run | Install/doctor outcome, named tests selected/executed/passed, time to first useful result |
| First mismatch | Expected/actual behavior, support classification, issue link and owner |
| Next step | Could the tester tell what to do next? Would they use this loop again, and for what? |

Start with 5–10 approved evaluators and report actual n/N outcomes. Try the
sample before a focused test in a real project the tester is authorized to use.
Fix repeated setup failures and false-success behavior before broader promotion.

## Pilot completion

The pilot is complete when the team can name which local paths are useful,
which paths remain advisory, which paths still require Salesforce, and what
evidence a future CI gate will retain.
