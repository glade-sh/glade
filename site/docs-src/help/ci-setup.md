---
pageType: troubleshooting
canonicalTask: /guide/workflows/ci
---

# Troubleshoot Glade CI setup

<div class="docs-intro">
  <p>Find the failed step, check its inputs, and recover the reports you need to diagnose it.</p>
</div>

## Diagnose a failed run

| Symptom | Check next |
| --- | --- |
| `glade: command not found` | Confirm installation succeeded and `$HOME/.local/bin` was added to `$GITHUB_PATH` before the Glade steps. |
| Project discovery fails | Run from the Salesforce DX project root. Use [the first local check](/guide/quickstart#_3-initialize-local-project-configuration) to create missing configuration safely, then rerun `glade doctor --project .`. |
| Changed-test selection cannot find its base | Confirm `origin/main` exists locally with `git rev-parse --verify origin/main`. Fetch the intended base and enough history; substitute your repository's actual base ref. |
| No tests are selected | Check the changed files and base ref. Follow [changed-test recovery](/help/changed-tests-before-pr); an empty selection does not prove the whole suite passes. |
| Reports are missing after a failed gate | Create `reports` before writing files and upload with `if: always()`. A step that never ran cannot produce its report. |

For a new workflow, start with [the CI quickstart](/guide/workflows/ci).
The illustrated setup below preserves the complete workflow and artifact steps.

The git examples assume `origin/main` is your intended base ref and is available
locally. Substitute the correct existing ref for your repository before running
changed-test or refactor commands.

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
      - run: curl -fsSL https://glade.sh/install.sh | env GLADE_VERSION=v0.2.15 sh
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
