# Help Screenshots

These images are real UI captures for `/help/...` articles.

Rules:

- Capture CLI images in a terminal.
- Capture editor images in a clean VS Code profile.
- Launch VS Code with `--user-data-dir` and `--extensions-dir`.
- Use `npm --prefix site run help:capture` to prepare the sample project and clean profile.
- The setup script does not open apps unless `OPEN_HELP_CAPTURE_APPS=1` is set.
- Install only Glade, Catppuccin for the Mocha theme, Salesforce Apex for Apex syntax highlighting, its Salesforce dependencies, and optional Salesforce pack extensions.
- Keep GitHub Chat and assistant sidebars closed.
- Use more than 100% UI zoom. Current captures use VS Code zoom `1.15` and terminal font size `18`.
- Keep final PNGs between `900x500` and `2200x1500` pixels so the docs lane does not shrink the UI.
- For Salesforce CLI local-target captures, use a disposable `HOME` plus `SF_USE_GENERIC_UNIX_KEYCHAIN=true` so macOS Keychain is not used.
- Do not show private orgs, private paths, tokens, email addresses, or unrelated extensions.
- Run `npm --prefix site run help:check` after capture.

## Prepare The Capture Project

From the worktree root:

```bash
cd /path/to/glade-worktree
export HELP_PROJECT_ROOT="/tmp/glade-help-capture/macrodata-apex"
npm --prefix site run help:capture
export REPO="$PWD"
export PROJECT="$HELP_PROJECT_ROOT"
export SHOT="$REPO/site/docs-src/public/help/screenshots"
export VSCODE_USER="${TMPDIR:-/tmp}/glade-help-vscode/user"
export VSCODE_EXTENSIONS="${TMPDIR:-/tmp}/glade-help-vscode/extensions"
```

`help:capture` installs the Glade VSIX, Catppuccin, and `salesforce.salesforcedx-vscode-apex` into that extension directory before the VS Code launch command uses it. VS Code also installs the Salesforce Core and Services dependencies required by the Apex extension.

Open a fresh terminal window when a recipe says "terminal":

```bash
cd "$PROJECT"
```

Open the clean VS Code profile when a recipe says "VS Code":

```bash
env \
  HOME="$PROJECT/.glade/sf-home" \
  SF_USE_GENERIC_UNIX_KEYCHAIN=true \
  SFDX_USE_GENERIC_UNIX_KEYCHAIN=true \
  SF_DISABLE_TELEMETRY=true \
  SFDX_DISABLE_TELEMETRY=true \
  SF_SKIP_NEW_VERSION_CHECK=true \
  code \
  --user-data-dir "$VSCODE_USER" \
  --extensions-dir "$VSCODE_EXTENSIONS" \
  --password-store=basic \
  --use-mock-keychain \
  --new-window "$PROJECT" \
  "$PROJECT/force-app/main/default/classes/RefinementServiceTest.cls"
```

Do not disable Salesforce Core or Services. The Salesforce Apex grammar depends on them, and the `.apex` scratch captures should show `Apex` in the status bar.

Check the clean profile extensions when the Apex highlighting looks wrong:

```bash
code --user-data-dir "$VSCODE_USER" --extensions-dir "$VSCODE_EXTENSIONS" --list-extensions
```

Use these capture rectangles from a second shell so the capture command itself is not visible:

```bash
# Terminal
screencapture -x -R80,80,954,472 "$SHOT/<name>.png"

# VS Code
screencapture -x -R80,80,1100,750 "$SHOT/<name>.png"
```

Use `printf '\033c'` to reset a terminal window. Do not use `clear`.

## Screenshot Recipes

Each screenshot can be recaptured with one command from the worktree root:

```bash
npm --prefix site run help:screenshot -- first-local-check-01-doctor
npm --prefix site run help:screenshot -- first-local-check-02-check
npm --prefix site run help:screenshot -- run-one-apex-test-01-cli
npm --prefix site run help:screenshot -- run-one-apex-test-02-codelens
npm --prefix site run help:screenshot -- run-one-apex-test-03-test-explorer
npm --prefix site run help:screenshot -- debug-apex-vscode-01-breakpoint
npm --prefix site run help:screenshot -- debug-apex-vscode-02-debug-toolbar
npm --prefix site run help:screenshot -- debug-apex-vscode-03-variables
npm --prefix site run help:screenshot -- anonymous-apex-scratch-01-buffer
npm --prefix site run help:screenshot -- anonymous-apex-scratch-02-run
npm --prefix site run help:screenshot -- local-data-environments-01-sidebar
npm --prefix site run help:screenshot -- local-data-environments-02-terminal
npm --prefix site run help:screenshot -- changed-tests-before-pr-01-changed-tests
npm --prefix site run help:screenshot -- changed-tests-before-pr-02-reports
npm --prefix site run help:screenshot -- glade-org-sf-data-import-01-create-start
npm --prefix site run help:screenshot -- glade-org-sf-data-import-02-auth-list
npm --prefix site run help:screenshot -- glade-org-sf-data-import-03-import-query
npm --prefix site run help:screenshot -- profile-apex-debug-log-01-profile
npm --prefix site run help:screenshot -- profile-apex-debug-log-02-json
npm --prefix site run help:screenshot -- ci-setup-01-workflow
npm --prefix site run help:screenshot -- ci-setup-02-artifacts
```

Use `npm --prefix site run help:screenshot -- --list` to print target names. Use `npm --prefix site run help:screenshot -- --all` to recapture the full set.

### first-local-check-01-doctor.png

Article: `first-local-check.md`.

In a fresh terminal window:

```bash
printf '\033c'
glade version
glade doctor
```

Capture from a second shell:

```bash
screencapture -x -R80,80,954,472 "$SHOT/first-local-check-01-doctor.png"
```

### first-local-check-02-check.png

Article: `first-local-check.md`.

In a fresh terminal window:

```bash
printf '\033c'
test -f glade.yml || glade init --project . --yes
glade config validate --project .
glade check --project . --no-progress
```

Capture from a second shell:

```bash
screencapture -x -R80,80,954,472 "$SHOT/first-local-check-02-check.png"
```

### run-one-apex-test-01-cli.png

Article: `run-one-apex-test.md`.

In a fresh terminal window:

```bash
printf '\033c'
glade test --project . --class RefinementServiceTest --no-progress
```

Capture from a second shell:

```bash
screencapture -x -R80,80,954,472 "$SHOT/run-one-apex-test-01-cli.png"
```

### run-one-apex-test-02-codelens.png

Article: `run-one-apex-test.md`.

In the clean VS Code profile:

```bash
code --user-data-dir "$VSCODE_USER" --extensions-dir "$VSCODE_EXTENSIONS" --reuse-window "$PROJECT/force-app/main/default/classes/RefinementServiceTest.cls"
```

Wait for the `Run Local Test` CodeLens above the class or method. Hide extra panels. Capture from a second shell:

```bash
screencapture -x -R80,80,1100,750 "$SHOT/run-one-apex-test-02-codelens.png"
```

### run-one-apex-test-03-test-explorer.png

Article: `run-one-apex-test.md`.

In the clean VS Code profile, open `RefinementService.cls`, set a breakpoint on `insert account;`, then start `Debug opensFile (macrodata-apex)` or `Debug Local Test` for `RefinementServiceTest.opensFile`. Capture after VS Code switches to Run and Debug and Variables shows `account` and `name`:

```bash
screencapture -x -R80,80,1100,750 "$SHOT/run-one-apex-test-03-test-explorer.png"
```

### debug-apex-vscode-01-breakpoint.png

Article: `debug-apex-vscode.md`.

In the clean VS Code profile:

```bash
code --user-data-dir "$VSCODE_USER" --extensions-dir "$VSCODE_EXTENSIONS" --reuse-window "$PROJECT/force-app/main/default/classes/RefinementService.cls"
```

Set a gutter breakpoint on `insert account;`. Keep the editor focused. Capture from a second shell:

```bash
screencapture -x -R80,80,1100,750 "$SHOT/debug-apex-vscode-01-breakpoint.png"
```

### debug-apex-vscode-02-debug-toolbar.png

Article: `debug-apex-vscode.md`.

In the clean VS Code profile:

```bash
code --user-data-dir "$VSCODE_USER" --extensions-dir "$VSCODE_EXTENSIONS" --reuse-window "$PROJECT/force-app/main/default/classes/RefinementServiceTest.cls"
```

Click `Debug Local Test` from CodeLens. Capture while the debug toolbar is visible:

```bash
screencapture -x -R80,80,1100,750 "$SHOT/debug-apex-vscode-02-debug-toolbar.png"
```

### debug-apex-vscode-03-variables.png

Article: `debug-apex-vscode.md`.

With the same debug session stopped at the breakpoint, open Run and Debug. Keep Variables and Call Stack visible, with the bottom panel closed. Capture from a second shell:

```bash
screencapture -x -R80,80,1100,750 "$SHOT/debug-apex-vscode-03-variables.png"
```

### anonymous-apex-scratch-01-buffer.png

Article: `anonymous-apex-scratch.md`.

In the clean VS Code profile:

```bash
code --user-data-dir "$VSCODE_USER" --extensions-dir "$VSCODE_EXTENSIONS" --reuse-window "$PROJECT/anonymous.apex"
```

Hide the Explorer if the code area is too narrow. Capture from a second shell:

```bash
screencapture -x -R80,80,1100,750 "$SHOT/anonymous-apex-scratch-01-buffer.png"
```

### anonymous-apex-scratch-02-run.png

Article: `anonymous-apex-scratch.md`.

From a terminal, regenerate the debug log if needed:

```bash
printf '\033c'
glade exec --project . --debug-log reports/anonymous-output.txt "$(cat anonymous.apex)"
```

Then open the log in the clean VS Code profile:

```bash
code --user-data-dir "$VSCODE_USER" --extensions-dir "$VSCODE_EXTENSIONS" --reuse-window "$PROJECT/reports/anonymous-output.txt"
```

Hide the Explorer if the log area is too narrow. Capture from a second shell:

```bash
screencapture -x -R80,80,1100,750 "$SHOT/anonymous-apex-scratch-02-run.png"
```

### local-data-environments-01-sidebar.png

Article: `local-data-environments.md`.

In the clean VS Code profile, open the Glade Activity Bar, then open Data Environments and Local Org. Keep only the Glade side view open. Capture from a second shell:

```bash
screencapture -x -R80,80,1100,750 "$SHOT/local-data-environments-01-sidebar.png"
```

### local-data-environments-02-terminal.png

Article: `local-data-environments.md`.

In a fresh terminal window:

```bash
printf '\033c'
mkdir -p .glade/envs
glade db seed --db .glade/envs/dev.sqlite --project . --no-progress seed.json
glade db inspect --db .glade/envs/dev.sqlite --project .
```

Capture from a second shell:

```bash
screencapture -x -R80,80,954,472 "$SHOT/local-data-environments-02-terminal.png"
```

### changed-tests-before-pr-01-changed-tests.png

Article: `changed-tests-before-pr.md`.

In a fresh terminal window:

```bash
printf '\033c'
glade test changed --project . --since origin/main --no-progress
```

If the fixture branch has no merge base, use the current base branch ref in place of `origin/main`. Capture from a second shell:

```bash
screencapture -x -R80,80,954,472 "$SHOT/changed-tests-before-pr-01-changed-tests.png"
```

### changed-tests-before-pr-02-reports.png

Article: `changed-tests-before-pr.md`.

In a fresh terminal window:

```bash
printf '\033c'
mkdir -p reports
glade test changed --project . --since origin/main --json --no-progress > reports/glade-test-changed.json
glade test --project . --class RefinementServiceTest --junit reports/glade-junit.xml --no-progress
wc -c reports/glade-test-changed.json reports/glade-junit.xml
```

Capture from a second shell:

```bash
screencapture -x -R80,80,1100,472 "$SHOT/changed-tests-before-pr-02-reports.png"
```

### glade-org-sf-data-import-01-create-start.png

Article: `glade-org-sf-data-import.md`.

In terminal window A:

```bash
printf '\033c'
rm -rf .glade/orgs/refine-local* .glade/sf-home
glade org create refine-local --project .
glade org start refine-local --project .
```

`glade org start` keeps running. Capture window A from a second shell:

```bash
screencapture -x -R80,80,954,472 "$SHOT/glade-org-sf-data-import-01-create-start.png"
```

### glade-org-sf-data-import-02-auth-list.png

Article: `glade-org-sf-data-import.md`.

Leave terminal window A running. In terminal window B:

```bash
printf '\033c'
export GLADE_SF_HOME="$PWD/.glade/sf-home"
mkdir -p "$GLADE_SF_HOME"
export HOME="$GLADE_SF_HOME"
export SF_USE_GENERIC_UNIX_KEYCHAIN=true
export SFDX_USE_GENERIC_UNIX_KEYCHAIN=true
export SF_DISABLE_TELEMETRY=true
export SF_SKIP_NEW_VERSION_CHECK=true
glade org auth refine-local --project .
sf org list
```

Capture window B from a second shell:

```bash
screencapture -x -R80,80,954,472 "$SHOT/glade-org-sf-data-import-02-auth-list.png"
```

### glade-org-sf-data-import-03-import-query.png

Article: `glade-org-sf-data-import.md`.

In the same terminal window B:

```bash
printf '\033c'
export GLADE_SF_HOME="$PWD/.glade/sf-home"
export HOME="$GLADE_SF_HOME"
export SF_USE_GENERIC_UNIX_KEYCHAIN=true
export SFDX_USE_GENERIC_UNIX_KEYCHAIN=true
export SF_DISABLE_TELEMETRY=true
export SF_SKIP_NEW_VERSION_CHECK=true
sf data import tree --plan data/insertOrder.json --target-org refine-local
sf data query --query "SELECT Id, Name FROM Account" --target-org refine-local
```

Capture window B from a second shell:

```bash
screencapture -x -R80,80,954,472 "$SHOT/glade-org-sf-data-import-03-import-query.png"
```

Stop the local org server in terminal window A when the last `sf` screenshot is captured.

### profile-apex-debug-log-01-profile.png

Article: `profile-apex-debug-log.md`.

In a fresh terminal window:

```bash
printf '\033c'
profile_md="$(glade debug profile --log reports/anonymous-output.txt --format markdown 2>/dev/null)"
printf "%s\n" "$profile_md" | awk '
  /^# glade profile/ || /^Events:/ || /^SOQL:/ || /^DML:/ || /^Email:/ || /^CPU:/ { print }
  /^## Hot events/ { hot=1; rows=0; print; next }
  hot && /^[|]/ { print; rows += 1; if (rows >= 4) exit }
'
```

Capture from a second shell:

```bash
screencapture -x -R80,80,1100,472 "$SHOT/profile-apex-debug-log-01-profile.png"
```

### profile-apex-debug-log-02-json.png

Article: `profile-apex-debug-log.md`.

In a fresh terminal window:

```bash
printf '\033c'
mkdir -p reports
export PROFILE=reports/apex-debug-profile.json
glade debug profile --log reports/anonymous-output.txt --json > "$PROFILE"
node -e 'const fs=require("fs"); const p=JSON.parse(fs.readFileSync(process.env.PROFILE,"utf8")); console.log(JSON.stringify({status:p.status,events:p.summary.events,dml:p.summary.limits.dml,dmlRows:p.summary.limits.dmlRows,hot:p.data.hot.map((event)=>event.name)}, null, 2));'
```

Capture from a second shell:

```bash
screencapture -x -R80,80,1100,472 "$SHOT/profile-apex-debug-log-02-json.png"
```

### ci-setup-01-workflow.png

Article: `ci-setup.md`.

In a fresh terminal window:

```bash
printf '\033c'
sed -n '1,10p;14,17p' .github/workflows/glade.yml
```

Capture from a second shell:

```bash
screencapture -x -R80,80,1100,472 "$SHOT/ci-setup-01-workflow.png"
```

### ci-setup-02-artifacts.png

Article: `ci-setup.md`.

In a fresh terminal window:

```bash
printf '\033c'
mkdir -p reports
CHECK=reports/glade-check.sarif
CHANGED=reports/glade-test-changed.json
JUNIT=reports/glade-junit.xml
glade check --project . --format sarif --output "$CHECK" --no-progress >/dev/null
glade test changed --project . --since origin/main --json --no-progress > "$CHANGED"
glade test --project . --junit "$JUNIT" --no-progress >/dev/null
wc -c "$CHECK" "$CHANGED" "$JUNIT"
```

Capture from a second shell:

```bash
screencapture -x -R80,80,1100,472 "$SHOT/ci-setup-02-artifacts.png"
```
