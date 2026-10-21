---
pageType: tutorial
canonicalTask: /guide/quickstart
outline: [2, 3]
---

# Five-minute Quickstart

<div class="docs-intro">
  <p class="docs-intro-eyebrow">Tutorial</p>
  <p>Install Glade, initialize one Salesforce DX project, and execute one named local Apex test.</p>
  <ul>
    <li>Try the disposable sample when you do not have a project ready.</li>
    <li>Use the project route when you already have Apex tests.</li>
    <li>Keep the local result separate from Salesforce validation.</li>
  </ul>
</div>

Packaged releases support macOS and Linux on `amd64` and `arm64`. Use a
terminal; neither route requires a Salesforce login.

## 1. Install and verify

```bash
curl -fsSL https://glade.sh/install.sh | sh
glade version
```

Expected: `glade version` prints the installed release. If the shell cannot
find Glade, add the default install directory for this session and try again:

```bash
export PATH="$HOME/.local/bin:$PATH"
glade version
```

Choose one route below.

## Route A: Try the sample {#sample-project}

::: info Stable sample
The corrected bundled test below first shipped in v0.2.14 and is included
in the v0.3.0 release. Do not
treat a playground Pass with source errors as successful validation.
:::

The bundled `RefinementServiceTest` first shipped in v0.2.14 and is included in
the v0.2.15 stable release. You can open that larger browser example after the
terminal-only first run below.

### 2A. Create the sample project

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
GLADE_PROJECT="$GLADE_SAMPLE_DIR"
```

Expected: a new temporary project containing `Sample.add` and
`SampleTest.adds`. It uses Apex source API `65.0` and does not set an LWC bundle
or local HTTP endpoint version.

### 3A. Initialize local project configuration {#_3-initialize-local-project-configuration}

```bash
test -f "$GLADE_PROJECT/glade.yml" || glade init --project "$GLADE_PROJECT" --yes
glade config validate --project "$GLADE_PROJECT"
glade doctor --project "$GLADE_PROJECT"
```

Expected: initialization creates `glade.yml` without overwriting an existing
file. Doctor identifies the project and parser and ends with `Ready.`. Fix the
first failed row before continuing.

### 4A. Check and run one named test

```bash
glade check --project "$GLADE_PROJECT"
glade test --project "$GLADE_PROJECT" --class SampleTest --method adds --json --no-progress
```

Expected: the check is clean. The test result names `SampleTest.adds`.
`total` and `passed` are `1`, while `failed`, `errors`, and `unsupported` are
`0`. Zero tests is not a successful first-run result.

To explore the larger bundled example in the browser workbench later:

```bash
GLADE_EXAMPLE_DIR="$(mktemp -d)"
glade playground --data-root "$GLADE_EXAMPLE_DIR/playground" --db "$GLADE_EXAMPLE_DIR/org.sqlite" --example refinement-service --open
```

Stop the browser workbench with Ctrl-C. The next Glade release adds a
no-server project materializer. Until that binary is published, keep using the
stable `--open` route above.

## Route B: Use my Salesforce DX project

### 2B. Enter the project

```bash
cd path/to/salesforce-dx-project
test -f sfdx-project.json
```

Expected: the file check exits `0`. If it fails, move to the directory that
owns `sfdx-project.json`.

### 3B. Initialize and diagnose {#initialize-existing-project}

```bash
test -f glade.yml || glade init --project . --yes
glade config validate --project .
glade doctor --project .
```

Review a new `glade.yml` before committing it. Doctor should identify the
project root, project source API default, parser, optional LWC toolchain, and
local-data state, then end with `Ready.` for Apex check and test.

### 4B. Check and run one project test

```bash
glade check --project .
glade test --project . --class <YourTestClass> --json --no-progress
```

Choose an existing `@IsTest` class. A clean check exits `0`; a source diagnostic
names its file and line and exits `1`. For the test, require at least one named
method in the result and inspect `failed`, `errors`, and `unsupported` as well
as the process exit code. Run the whole suite only when that is the intended
next step:

```bash
glade test --project .
```

## API versions are separate contracts

| Version axis | Current local contract |
| --- | --- |
| Apex source | 62.0-68.0 |
| Historical Apex source below 62.0 | Unsupported, even if a well-formed version is preserved by the loader |
| Execute Anonymous | 65.0, 66.0, 67.0, 68.0 |
| LWC bundles | 59.0-68.0; explicit whole versions required when declared; unversioned compiler default 68.0 |
| Visualforce | 59.0-68.0 |
| Local HTTP endpoints | 60.0, 65.0, 66.0, 67.0, 68.0; default 65.0 |

The v0.3.0 support window is Apex 62.0-68.0, LWC and Visualforce 59.0-68.0.
A clean check or test on an older project does not make Execute Anonymous
eligible at that version. An HTTP endpoint version does not change source
semantics. Do not bump project metadata merely to obtain a green result.
Use the [support map](/guide/support-map) for behavior-specific limits.

## What the local result proves

Glade read project files and ran supported behavior in its local runtime. Glade
did not log in to Salesforce, deploy metadata, evaluate org permissions,
contact a hosted service, or prove production parity. Use the
[support map](/guide/support-map) for capability-specific limits. Then retain Salesforce deployment and tests for
final validation.

## Clean up or continue

`glade init` creates only `glade.yml`. The sample route writes its files under
the temporary evaluation directory; opening the workbench can also create a
managed workspace and SQLite file there. Print and review the sample directory
before moving it to Trash or deleting it:

```bash
printf '%s\n' "$GLADE_SAMPLE_DIR"
printf '%s\n' "${GLADE_EXAMPLE_DIR:-}"
```

For a kept project, clear only Glade's project-local test startup cache after a
branch switch or stale result:

```bash
glade test clear-cache --project .
```

- [Run Apex tests](/guide/workflows/apex-tests)
- [Debug Apex](/guide/workflows/debug-apex)
- [Execute Apex and SOQL](/guide/playground)
- [Report a reproducible issue](https://github.com/glade-sh/glade/issues)
