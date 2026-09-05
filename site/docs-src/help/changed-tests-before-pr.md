# Run Changed Tests Before a PR

<div class="docs-intro">
  <p class="docs-intro-eyebrow">Task guide</p>
  <p>Run the local tests Glade can connect to your branch diff, then save machine-readable output.</p>
  <ul>
    <li>Run changed tests against `origin/main`.</li>
    <li>Rerun failures.</li>
    <li>Write reports for review or CI.</li>
  </ul>
</div>

## Before you start

- Your branch has access to `origin/main`.
- `reports/` exists before commands write files there.
- Screenshots for this article are captured in a terminal.

## Steps

### 1. Run changed tests

```bash
glade test changed --project . --since origin/main --no-progress
```

Expected: Glade selects the smallest test set it can prove from the diff and prints a local pass/fail summary.

![Terminal showing changed local tests](/help/screenshots/changed-tests-before-pr-01-changed-tests.png)

### 2. Save review artifacts

```bash
mkdir -p reports
glade test changed --project . --since origin/main --json --no-progress > reports/glade-test-changed.json
glade test --project . --junit reports/glade-junit.xml --no-progress
wc -c reports/glade-test-changed.json reports/glade-junit.xml
```

Expected: `reports/` contains JSON and JUnit output for PR review or CI upload.

![Terminal showing Glade report files](/help/screenshots/changed-tests-before-pr-02-reports.png)

## Common wrong turn

The comparison ref must exist locally. Fetch it before running changed-test
selection; in GitHub Actions, use `fetch-depth: 0`. Glade compares against the
specified ref directly; it does not calculate a merge base.

## Next

- [Add Glade to CI](/help/ci-setup)
- [Affected tests](/guide/affected-tests)
