# Run your first local Apex check

<div class="docs-intro">
  <p class="docs-intro-eyebrow">Tutorial</p>
  <p>Prove that Glade can discover a Salesforce DX project and return a useful local result before an org round trip.</p>
  <ul>
    <li>Verify the binary.</li>
    <li>Establish project context.</li>
    <li>Run one check and interpret the result.</li>
  </ul>
</div>

## Prerequisites

- macOS or Linux for a packaged release
- a Salesforce DX project with `sfdx-project.json`
- Apex source in a configured package directory

## 1. Install and verify Glade

```bash
curl -fsSL https://glade.sh/install.sh | sh
glade version
```

Expected: `glade version` prints the installed stable version. If the shell
cannot find `glade`, add `~/.local/bin` to `PATH` and open a new shell.

## 2. Enter the project

```bash
cd path/to/salesforce-dx-project
test -f sfdx-project.json
```

Expected: the file check exits with code `0` and prints nothing.

## 3. Initialize local project configuration

```bash
glade init --project . --yes
glade config validate --project .
```

Expected: Glade writes `glade.yml` when it is absent and config validation
exits with code `0`. Review a new `glade.yml` before committing it.

## 4. Check the project-aware environment

```bash
glade doctor --project .
```

Expected: Glade reports the project, parser, toolchain, config, and runtime,
then ends with `Ready.`. A missing project or parser is a setup failure; fix it
before treating later diagnostics as Apex results.

## 5. Run one local check

```bash
glade check --project .
```

Expected:

- a clean result and exit code `0`; or
- a named diagnostic with file and line, plus exit code `1`.

A named source diagnostic is a valid local result. It is different from a
setup error such as an undiscovered project or unavailable parser.

## 6. Run discovered tests

```bash
glade test --project .
```

Expected: a selected/passed/failed summary, with file and method details for a
failure. After the first run, narrow the loop to a class that exists in your
project:

```bash
glade test --project . --class <YourTestClass>
```

## Local result boundary

This path runs against supported local project state. Use Salesforce for
hosted services, deployment, and final production validation. Check the
[versioned support map](/guide/support-map) for the exact capability.

## You are done when

Glade discovers the project, reports the expected source path, and returns
either a clean local check or a named diagnostic with a file and line.

## Reset or clean up

`glade init` writes only the local project configuration. If this was an
evaluation and you do not want to keep it, review and remove `glade.yml`. Clear
project-local startup state after a branch switch or stale result:

```bash
glade test clear-cache --project .
```

## Choose the next workflow

- [Run Apex tests](/guide/workflows/apex-tests)
- [Debug Apex](/guide/workflows/debug-apex)
- [Execute Apex and SOQL](/guide/workbench#exec)
- [Work with local data](/guide/workflows/local-data)
