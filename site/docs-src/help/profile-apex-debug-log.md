---
pageType: guide
canonicalTask: /guide/workflows/debug-apex
---

# Profile an Apex debug log

<div class="docs-intro">
  <p class="docs-intro-eyebrow">Task guide</p>
  <p>Turn a Salesforce Apex debug log into a local profile report.</p>
  <ul>
    <li>Run <code>glade debug profile</code> on a saved log.</li>
    <li>Read limits, categories, and Hot events.</li>
    <li>Save JSON for automation or follow-up review.</li>
  </ul>
</div>

For the main task path, use [the guide](/guide/workflows/debug-apex). This walkthrough keeps the
illustrated steps and recovery details for this interface.

## Before you start

- You have a Salesforce debug log saved on disk.
- Run from the Salesforce DX project root when you want project context nearby.
- You have a terminal open at the project root.
- The log does not contain secrets you should not store in local reports.

## Steps

### 1. Profile the log

```bash
glade debug profile --log <debug-log> --format markdown
```

Expected: Glade prints a runtime summary, limit counts, categories, and Hot events.

![Terminal showing an Apex debug log profile](/help/screenshots/profile-apex-debug-log-01-profile.png)

### 2. Save JSON for automation

```bash
mkdir -p reports
glade debug profile --log <debug-log> --json > reports/apex-debug-profile.json
```

Expected: the JSON report records the profile status, summary limits, and hot event rows.

![Terminal showing Apex debug log profile JSON](/help/screenshots/profile-apex-debug-log-02-json.png)

## Common wrong turn

`glade profile analyze` reads Glade native trace JSON from `glade test --trace`. Use `glade debug profile` for Salesforce Apex debug logs.

## Next

- [Use anonymous Apex scratch in VS Code](/help/anonymous-apex-scratch)
- [CLI reference](/reference/cli)
