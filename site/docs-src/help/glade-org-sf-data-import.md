# Set Up a Glade Org and Import Data With sf

<div class="docs-intro">
  <p class="docs-intro-eyebrow">Guided help</p>
  <p>Create a local Glade org target, register it with the Salesforce CLI, and import sample data.</p>
  <ul>
    <li>Create and start a Glade org.</li>
    <li>Write the target into an isolated `sf` config.</li>
    <li>Import tree data and query it back.</li>
  </ul>
</div>

## Before you start

- `glade doctor` passes from the SFDX project root.
- The Salesforce CLI `sf` is installed.
- The project has a Salesforce tree import plan and data files.
- Pick a local target alias and use it consistently in every command below.
- Screenshots for this article are captured in a terminal.
- This creates a local Glade target. It does not create a Salesforce scratch org.
- For this walkthrough, use a disposable Salesforce CLI home so the local target does not touch your real org list or macOS Keychain.

## Steps

### 1. Create and start the Glade org

```bash
glade org create <local-target> --project .
glade org start <local-target> --project .
```

Expected: Glade writes `.glade/orgs/<local-target>.sqlite` and starts a loopback server for the saved target.

![Terminal showing glade org create and start output](/help/screenshots/glade-org-sf-data-import-01-create-start.png)

### 2. Register the target with sf

Open a second fresh terminal window in the same project and run:

```bash
export GLADE_SF_HOME="$PWD/.glade/sf-home"
mkdir -p "$GLADE_SF_HOME"
export HOME="$GLADE_SF_HOME"
export SF_USE_GENERIC_UNIX_KEYCHAIN=true
export SFDX_USE_GENERIC_UNIX_KEYCHAIN=true
export SF_DISABLE_TELEMETRY=true
export SF_SKIP_NEW_VERSION_CHECK=true
glade org auth <local-target> --project .
sf org list
```

Expected: Glade writes a local Salesforce CLI authorization, `sf` reports `Successfully authorized`, and `sf org list` shows only the local `<local-target>` target.

![Terminal showing local sf authorization for a Glade target](/help/screenshots/glade-org-sf-data-import-02-auth-list.png)

### 3. Import and query data

```bash
sf data import tree --plan <plan-file> --target-org <local-target>
sf data query --query "SELECT Id, Name FROM Account" --target-org <local-target>
```

Expected: the import succeeds and the query returns the local rows.

![Terminal showing sf data import tree and query output](/help/screenshots/glade-org-sf-data-import-03-import-query.png)

## Common wrong turn

If `sf` cannot find the target, make sure the same terminal window still has `HOME="$PWD/.glade/sf-home"` and `SF_USE_GENERIC_UNIX_KEYCHAIN=true` set.

## Next

- [Use Glade as an sf target](/guide/glade-orgs)
- [Local API routes](/guide/local-api-server)
