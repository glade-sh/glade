---
pageType: tutorial
outline: [2, 3]
canonicalTask: /guide/quickstart
---

# Run your first local Apex check

<div class="docs-intro">
  <p class="docs-intro-eyebrow">Tutorial</p>
  <p>Run a real local Apex test, inspect the result, and keep Salesforce for final validation. Start with the bundled sample, a minimal terminal sample, or your own Salesforce DX project.</p>
</div>

## Prerequisites

Packaged releases target macOS and Linux on arm64 and amd64. No Salesforce
login is needed for these local steps. Native Windows archives are not
published; WSL is not a verified platform for this walkthrough. Run project
commands from the directory containing your Salesforce DX project, or use the
explicit sample workspace path below.

## 1. Install and verify Glade

```bash
curl -fsSL https://glade.sh/install.sh | sh
export PATH="$HOME/.local/bin:$PATH"
glade version
```

Expected: `glade version` prints the installed version. Save the PATH line in
your shell configuration to keep it in new terminals.

See [Installation](/guide/installation) for version pinning and archive verification.

::: info Stable sample
The corrected bundled test below first shipped in v0.2.14 and is included
in the v0.2.15 stable release. Do not
treat a playground Pass with source errors as successful validation.
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
test -f .glade/playground/workspaces/default/glade.yml || glade init --project .glade/playground/workspaces/default --yes
glade config validate --project .glade/playground/workspaces/default
glade doctor --project .glade/playground/workspaces/default
glade check --project .glade/playground/workspaces/default --json
glade test --project .glade/playground/workspaces/default --class RefinementServiceTest --json
```

Expected: initialization creates `glade.yml` if it is absent; config validation
passes; doctor identifies the sample and parser and ends with `Ready.`; check has no diagnostics; and
`RefinementServiceTest.createsAndLabelsFileRow` passes. The executed test total
must be at least one. A successful process with **zero tests** is not a
successful first-test result.

To see the feedback loop, change the test's expected label to an incorrect
value, rerun the same command, read the assertion failure, then restore
`Refine 01 #F-100` and rerun. Change only your evaluation copy, not business
code to accommodate a runtime limitation.

`glade examples run` prints an opening command; it does not export a project
or execute tests. The commands above use the workspace loaded by the browser.

## Sample project {#sample-project}

Prefer a terminal-only sample? Create a fresh scratch directory with the same
two Apex classes used by Glade's
[runtime smoke check](https://github.com/glade-sh/glade/blob/main/scripts/smoke-runtime.sh):

```bash
GLADE_SAMPLE_DIR="$(mktemp -d)"
cd "$GLADE_SAMPLE_DIR"
mkdir -p force-app/main/classes
cat > sfdx-project.json <<'JSON'
{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"65.0"}
JSON
cat > force-app/main/classes/Sample.cls <<'APEX'
public class Sample {
  public static Integer add(Integer a, Integer b) {
    return a + b;
  }
}
APEX
cat > force-app/main/classes/SampleTest.cls <<'APEX'
@isTest
private class SampleTest {
  @isTest static void adds() {
    System.assertEquals(3, Sample.add(1, 2));
  }
}
APEX
```

Expected: a new temporary project with `SampleTest.adds`. Stay in this directory
and continue at [step 3](#_3-initialize-local-project-configuration). This sample
uses Apex source API 65.0; it does not set an LWC component or HTTP endpoint
version.

## Use my Salesforce DX project

For an existing project, start at step 2. If you created the terminal sample
above, stay in its directory and start at step 3. The bundled Playground sample
already ran these checks against its explicit workspace path.

### 2. Enter the project

```bash
cd path/to/salesforce-dx-project
test -f sfdx-project.json
```

Expected: the file check exits with code `0` and prints nothing.

### 3. Initialize local project configuration

```bash
test -f glade.yml || glade init --project . --yes
glade config validate --project .
```

The file check preserves an existing `glade.yml`. Do not add `--force` during
first-run setup. Expected: initialization creates `glade.yml` only if absent;
config validation exits with code `0`. Review a new configuration before
committing it.

### 4. Check the project-aware environment

```bash
glade doctor --project .
```

Expected: doctor reports the discovered project and parser and ends with
`Ready.`. A missing project or parser is a setup failure, not an Apex result.

### 5. Run one local check

```bash
glade check --project .
```

Expected: a clean result and exit code `0`, or a named diagnostic with a file
and line and exit code `1`. A named source diagnostic is different from a
setup error such as an undiscovered project or unavailable parser.

### 6. Run discovered tests

For your own project, start with one test class that already exists. This
command uses `RefinementServiceTest` as an example;
substitute your actual class name:

```bash
glade test --project . --class RefinementServiceTest --json --no-progress
```

For the terminal sample, select the exact method:

```bash
glade test --project . --class SampleTest --method adds --json --no-progress
```

Expected for the terminal sample: `SampleTest.adds` executes, `total` and
`passed` are `1`, and `failed`, `errors`, and `unsupported` are `0`. An empty
selection is not success evidence. For your own project, read the same counts
and method names; unsupported outcomes are unvalidated behavior and exit `1`.

Once the focused loop is useful, expand to `glade test --project .` or
[affected tests](/guide/affected-tests).

## Local result boundary

This path runs against supported local project state. Use Salesforce for
hosted services, deployment, and final production validation. Check the
[versioned support map](/guide/support-map) for the exact capability.

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
It is not a hidden Salesforce org or a complete hosted-platform emulator. At
least one named test must execute before calling the test setup complete.

## Reset or clean up

The bundled sample's source, SQLite database, and cache live inside the evaluation
folder's `.glade/`. Stop its server before moving that evaluation folder to
Trash. Do not remove a shared Glade home or another project's state.

For the terminal sample, keep the temporary directory path if you want to
inspect or remove it later. Review that directory before removing it.

For your own project, review the newly created `glade.yml` and `.glade/`
files before deciding what to retain. To clear test startup state only:

```bash
glade test clear-cache --project .
```

<span id="choose-the-next-workflow"></span>

## Give feedback and choose the next workflow

[Report a bug or describe your workflow](https://github.com/glade-sh/glade/issues/new/choose).
Include version, OS, command, expected/actual result, test count, and a minimal
public reproduction. Do not include proprietary source, private package names,
credentials, or customer records. Use [private security reporting](/guide/security-trust)
for vulnerabilities.

- [Run Apex tests](/guide/workflows/apex-tests)
- [Debug Apex](/guide/workflows/debug-apex)
- [Execute Apex and SOQL](/guide/playground)
- [Work with local data](/guide/workflows/local-data)
