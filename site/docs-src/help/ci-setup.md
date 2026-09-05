# Troubleshoot Glade CI setup

<div class="docs-intro">
  <p class="docs-intro-eyebrow">Task guide</p>
  <p>Add Glade checks, affected tests, JUnit, SARIF, and artifacts to a pull request workflow.</p>
  <ul>
    <li>Fetch full git history for changed-test selection.</li>
    <li>Run `glade check`, changed tests, and JUnit output.</li>
    <li>Upload reports even when a gate fails.</li>
  </ul>
</div>

## Before you start

- The repository uses GitHub Actions.
- The workflow can install Glade from `https://glade.sh/install.sh`.
- The project can run local checks and tests from a terminal.

## Steps

### 1. Add the workflow

For an evaluation, start with the [advisory pilot](/guide/ci-artifacts#advisory-pilot).
The example below is an enforcing gate: check/test failures fail the job.
Create `.github/workflows/glade.yml`:

```yaml
name: glade
on: [pull_request]

jobs:
  glade:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      - run: curl -fsSL https://glade.sh/install.sh | env GLADE_VERSION=v0.2.14 sh
      - run: echo "$HOME/.local/bin" >> "$GITHUB_PATH"
      - run: glade version
      - run: glade doctor --project .
      - run: mkdir -p reports
      - run: glade check --project . --format sarif --output reports/glade-check.sarif --no-progress
      - run: glade test changed --project . --since origin/main --json --no-progress > reports/glade-test-changed.json
      - run: glade test --project . --junit reports/glade-junit.xml --no-progress
```

Expected: the workflow has `fetch-depth: 0`, creates `reports`, and writes SARIF, JSON, and JUnit outputs.

![Terminal showing a Glade GitHub Actions workflow](/help/screenshots/ci-setup-01-workflow.png)

### 2. Prove the artifact commands locally

Run the same report-producing commands in a terminal:

```bash
mkdir -p reports
glade check --project . --format sarif --output reports/glade-check.sarif --no-progress
glade test changed --project . --since origin/main --json --no-progress > reports/glade-test-changed.json
glade test --project . --junit reports/glade-junit.xml --no-progress
wc -c reports/glade-check.sarif reports/glade-test-changed.json reports/glade-junit.xml
```

Expected: the report files exist and have non-zero size.

![Terminal showing Glade CI artifacts](/help/screenshots/ci-setup-02-artifacts.png)

### 3. Upload artifacts after failures

Add an upload step with `if: always()`:

```yaml
      - uses: actions/upload-artifact@v4
        if: always()
        with:
          name: glade-results
          path: |
            reports/glade-check.sarif
            reports/glade-test-changed.json
            reports/glade-junit.xml
```

Expected: failed checks still leave the report files attached to the workflow run.

## Common wrong turn

Changed-test selection needs the base ref on disk. Keep `fetch-depth: 0` when the workflow runs `glade test changed --since origin/main`.

## Next

- [Run changed tests before a PR](/help/changed-tests-before-pr)
- [Add Glade to CI](/guide/ci-artifacts)
