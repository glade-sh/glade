---
pageType: guide
canonicalTask: /guide/affected-tests
---

# Run changed tests before a PR

<div class="docs-intro">
  <p class="docs-intro-eyebrow">Task guide</p>
  <p>Run the local tests Glade can connect to your branch diff, then save machine-readable output.</p>
  <ul>
    <li>Run changed tests against <code>origin/main</code>.</li>
    <li>Rerun failures.</li>
    <li>Write reports for review or CI.</li>
  </ul>
</div>

For the main task path, use [the guide](/guide/affected-tests). This walkthrough keeps the
illustrated steps and recovery details for this interface.

The git examples assume `origin/main` is your intended base ref and is available
locally. Substitute the correct existing ref for your repository before running
changed-test or refactor commands.

## Before you start

- Your branch has access to `origin/main`.
- `reports/` exists before commands write files there.
- Run from the initialized Salesforce DX project root.

The screenshots for this article were captured in a terminal.

## Steps

### 1. Run changed tests

```bash
glade test changed --project . --since origin/main --no-progress
```

Expected: Glade selects tests conservatively from the diff and prints a local
summary. Read the executed count: zero selected tests may exit `0`; run an
explicit relevant class or suite when you need execution evidence.

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
