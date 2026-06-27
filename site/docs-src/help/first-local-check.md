# Install Glade and Run the First Local Check

<div class="docs-intro">
  <p class="docs-intro-eyebrow">Guided help</p>
  <p>Install Glade, prove the binary, and run the first local check from a terminal.</p>
  <ul>
    <li>Run `glade doctor`.</li>
    <li>Initialize an SFDX project.</li>
    <li>Read the first `glade check` result.</li>
  </ul>
</div>

## Before you start

- You have a terminal open at the project root.
- You have an SFDX project open at the project root.
- Your shell can find `glade` on `PATH`.

## Steps

### 1. Prove the install

```bash
glade version
glade doctor
```

Expected: `glade doctor` ends with `Ready.`

![Terminal showing glade doctor ready output](/help/screenshots/first-local-check-01-doctor.png)

### 2. Initialize and check the project

```bash
test -f glade.yml || glade init --project . --yes
glade config validate --project .
glade check --project .
```

Expected: `glade check` exits `0` for clean source or exits `1` with file and line diagnostics.

![Terminal showing glade check output](/help/screenshots/first-local-check-02-check.png)

## Common wrong turn

`glade: command not found` means the install directory is not on `PATH`. Add `~/.local/bin` to `PATH`, restart the terminal, and run `glade doctor` again.

## Next

- [Run one Apex test locally](/help/run-one-apex-test)
- [Quickstart reference](/guide/quickstart)
