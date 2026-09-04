# Run your first local Apex check

<div class="docs-intro">
  <p class="docs-intro-eyebrow">Tutorial</p>
  <p>Run a real local Apex test, inspect the result, and keep Salesforce for final validation. Start with the bundled sample or your own Salesforce DX project.</p>
</div>

## Prerequisites

Packaged releases target macOS and Linux on arm64 and amd64. No Salesforce
login is needed for these local steps. Native Windows archives are not
published; WSL is not a verified platform for this walkthrough.

## 1. Install and verify Glade

```bash
curl -fsSL https://glade.sh/install.sh | sh
export PATH="$HOME/.local/bin:$PATH"
glade version
```

Expected: `glade version` prints the installed version. Save the PATH line in
your shell configuration to keep it in new terminals.

See [Installation](/guide/installation) for version pinning and archive verification.

::: warning Sample version boundary
The corrected bundled test below is part of the v0.2.14 release candidate.
The published v0.2.13 `refinement-service` sample has a reserved-identifier
error and no test class. On v0.2.13, use the existing-project route below;
do not treat a playground Pass with source errors as successful validation.
:::

## Try the sample

Create a new evaluation folder. The first command deliberately fails if that
folder already exists; choose another empty folder rather than overwrite it.

```bash
mkdir macrodata-apex && cd macrodata-apex &&
  glade playground --data-root .glade/playground --db .glade/playground/org.sqlite --example refinement-service --open
```

The local Playground opens **Refinement Service**. Wait for its files to load
and confirm `RefinementServiceTest.cls` is present. The service inserts an
Account, SOQL reads it back, and `FileRow` formats its label.

Stop this server with Ctrl-C. From the same evaluation folder:

```bash
glade doctor --project .glade/playground/workspaces/default
glade check --project .glade/playground/workspaces/default --json
glade test --project .glade/playground/workspaces/default --class RefinementServiceTest --json
```

Expected: doctor identifies the sample and parser, check has no diagnostics,
and `RefinementServiceTest.createsAndLabelsFileRow` passes. The executed test
total must be at least one. A successful process with **zero tests** is not a
successful first-test result.

To see the feedback loop, change the test's expected label to an incorrect
value, rerun the same command, read the assertion failure, then restore
`Refine 01 #F-100` and rerun. Change only your evaluation copy, not business
code to accommodate a runtime limitation.

`glade examples run` prints an opening command; it does not export a project
or execute tests. The commands above use the workspace loaded by the browser.

## Use my Salesforce DX project

Enter the folder containing your existing `sfdx-project.json`, then:

```bash
test -f sfdx-project.json
test -f glade.yml || glade init --project . --yes
glade config validate --project .
glade doctor --project .
glade check --project .
```

Expected: initialization creates `glade.yml` if absent; review it before
committing. Doctor reports the discovered project and parser and ends with
`Ready.`. A missing project or parser is a setup failure, not an Apex result.

Start with one test class that already exists in your project. This command
uses `RefinementServiceTest` as an example; substitute your actual class name:

```bash
glade test --project . --class RefinementServiceTest --json --no-progress
```

Expected: a selected/passed/failed summary naming the executed tests. Once the
focused loop is useful, expand to `glade test --project .` or
[affected tests](/guide/affected-tests).

## Interpret the result

- **Clean check and named passing tests:** a useful local result for that path.
- **Named source diagnostic:** inspect the file and line. A diagnostic is not a passing test.
- **Missing source or package dependency:** configure the project inputs before assessing compatibility.
- **Unsupported behavior:** consult the support map and keep Salesforce as the validation gate; report an unexpected mismatch.

## API versions are separate contracts

| Axis | Checked contract |
| --- | --- |
| Apex source | 65.0, 66.0, 67.0 |
| Historical Apex source | Well-formed positive versions are preserved, outside checked correctness/parity credit |
| Execute Anonymous | Limited to the checked Apex source window |
| LWC bundles | Each bundle declares an exact supported version |
| Local HTTP endpoints | 60.0, 65.0, 66.0, 67.0; default 65.0 |

A clean check or test on an older project does not make Execute Anonymous
eligible at that version. An HTTP endpoint version does not change source
semantics. Do not bump project metadata merely to obtain a green result.
Use the [support map](/guide/support-map) for behavior-specific limits.

## You are done when

You can name the project, version, and test that ran, interpret its result,
and identify the Salesforce validation still required. Glade reads source and
metadata from disk and executes supported behavior in its own local runtime.
It is not a hidden Salesforce org or a complete hosted-platform emulator.

## Reset or clean up

The sample's source, SQLite database, and cache live inside the evaluation
folder's `.glade/`. Stop its server before moving that evaluation folder to
Trash. Do not remove a shared Glade home or another project's state.

For your own project, review the newly created `glade.yml` and `.glade/`
files before deciding what to retain. To clear test startup state only:

```bash
glade test clear-cache --project .
```

## Give feedback and choose the next workflow

[Report a bug or describe your workflow](https://github.com/glade-sh/glade/issues/new/choose).
Include version, OS, command, expected/actual result, test count, and a minimal
public reproduction. Do not include proprietary source, private package names,
credentials, or customer records. Use [private security reporting](/guide/security-trust)
for vulnerabilities.

- [Run Apex tests](/guide/workflows/apex-tests)
- [Debug Apex](/guide/workflows/debug-apex)
- [Execute Apex and SOQL](/help/anonymous-apex-scratch)
- [Work with local data](/guide/workflows/local-data)
