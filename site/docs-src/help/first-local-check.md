# Run the first local check

<div class="docs-intro">
  <p class="docs-intro-eyebrow">Guided help</p>
  <p>Initialize a Salesforce DX project, prove the local environment, and run the first local check from a terminal.</p>
  <ul>
    <li>Initialize a Salesforce DX project.</li>
    <li>Run `glade doctor`.</li>
    <li>Read the first `glade check` result.</li>
  </ul>
</div>

## Before you start

- You have a terminal open at the Salesforce DX project root.
- `glade version` works in that terminal. If it does not, [install Glade](/guide/installation) first.

## Steps

### 1. Initialize the project

```bash
test -f glade.yml || glade init --project . --yes
glade config validate --project .
glade doctor
```

Expected: `glade.yml` exists and `glade doctor` ends with `Ready.`

![Terminal showing glade doctor ready output](/help/screenshots/first-local-check-01-doctor.png)

### 2. Check the project

```bash
glade check --project .
```

Expected: `glade check` exits `0` for clean source or exits `1` with file and line diagnostics.

![Terminal showing glade check output](/help/screenshots/first-local-check-02-check.png)

## Common wrong turn

`glade: command not found` means the install directory is not on `PATH`. Add `~/.local/bin` to `PATH`, restart the terminal, and run `glade doctor` again.

## Next

- [Run one Apex test locally](/help/run-one-apex-test)
- [Quickstart reference](/guide/quickstart)
