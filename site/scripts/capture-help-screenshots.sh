#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SITE="$ROOT/site"
CAPTURE_ROOT="$ROOT/.glade/help-capture"
PROJECT_ROOT="${HELP_PROJECT_ROOT:-$ROOT/.glade/macrodata-apex}"
VSCODE_OPEN_FILE="$PROJECT_ROOT/force-app/main/default/classes/RefinementServiceTest.cls"
SCREENSHOT_DIR="$SITE/docs-src/public/help/screenshots"
VSCODE_PROFILE_ROOT="${TMPDIR:-/tmp}/glade-help-vscode"
VSCODE_USER="$VSCODE_PROFILE_ROOT/user"
VSCODE_EXTENSIONS="$VSCODE_PROFILE_ROOT/extensions"
VSCODE_WINDOW_ZOOM="${VSCODE_WINDOW_ZOOM:-1.15}"
VSCODE_EDITOR_FONT_SIZE="${VSCODE_EDITOR_FONT_SIZE:-16}"
VSCODE_CAPTURE_WIDTH="${VSCODE_CAPTURE_WIDTH:-1100}"
VSCODE_CAPTURE_HEIGHT="${VSCODE_CAPTURE_HEIGHT:-750}"
CATPPUCCIN_EXTENSION_ID="${CATPPUCCIN_EXTENSION_ID:-catppuccin.catppuccin-vsc}"
SALESFORCE_APEX_EXTENSION_ID="${SALESFORCE_APEX_EXTENSION_ID:-salesforce.salesforcedx-vscode-apex}"
GHOSTTY_FONT_SIZE="${GHOSTTY_FONT_SIZE:-18}"
GHOSTTY_WINDOW_WIDTH="${GHOSTTY_WINDOW_WIDTH:-96}"
GHOSTTY_WINDOW_HEIGHT="${GHOSTTY_WINDOW_HEIGHT:-28}"
SF_CAPTURE_HOME="$PROJECT_ROOT/.glade/sf-home"

mkdir -p "$CAPTURE_ROOT" "$SCREENSHOT_DIR" "$SF_CAPTURE_HOME"

project_path="$(HELP_PROJECT_ROOT="$PROJECT_ROOT" npm --prefix "$SITE" run --silent help:project)"
if [[ "$project_path" != "$PROJECT_ROOT" ]]; then
  echo "unexpected project path: $project_path" >&2
  exit 1
fi

npm --prefix "$ROOT/contrib/vscode-glade" run package
rm -rf "$VSCODE_PROFILE_ROOT"
mkdir -p "$VSCODE_USER/User" "$VSCODE_EXTENSIONS"

cat > "$VSCODE_USER/User/settings.json" <<'JSON'
{
  "workbench.startupEditor": "none",
  "workbench.restoreWindows": "none",
  "workbench.commandCenter": false,
  "workbench.colorTheme": "Catppuccin Mocha",
  "workbench.sideBar.location": "left",
  "workbench.activityBar.location": "hidden",
  "workbench.secondarySideBar.defaultVisibility": "hidden",
  "window.zoomLevel": 1.15,
  "editor.fontSize": 16,
  "editor.lineHeight": 24,
  "editor.minimap.enabled": false,
  "editor.renderWhitespace": "none",
  "debug.showInStatusBar": "never",
  "debug.autoExpandLazyVariables": "on",
  "files.associations": {
    "*.apex": "apex"
  },
  "workbench.editor.showTabs": "single",
  "workbench.welcomePage.walkthroughs.openOnInstall": false,
  "chat.agent.enabled": false,
  "chat.allowAnonymousAccess": true,
  "chat.commandCenter.enabled": false,
  "chat.titleBar.signIn.enabled": false,
  "github.copilot.chat.enabled": false,
  "github.copilot.nextEditSuggestions.enabled": false,
  "git.enabled": false,
  "git.openRepositoryInParentFolders": "never",
  "extensions.ignoreRecommendations": true,
  "update.showReleaseNotes": false,
  "terminal.integrated.initialHintCopilotCli": false,
  "security.workspace.trust.enabled": false,
  "telemetry.enableTelemetry": false,
  "telemetry.telemetryLevel": "off",
  "salesforcedx-vscode-core.telemetry.enabled": false,
  "salesforcedx-vscode-core.show-cli-success-msg": false,
  "salesforcedx-vscode-apex.advanced.enable-completion-statistics": false,
  "salesforcedx-vscode-apex.enable-apex-ls-error-to-telemetry": false
}
JSON

cat > "$VSCODE_USER/User/keybindings.json" <<'JSON'
[
  {
    "key": "cmd+alt+g",
    "command": "workbench.view.extension.glade"
  },
  {
    "key": "cmd+alt+e",
    "command": "glade.environments.focus"
  },
  {
    "key": "cmd+alt+n",
    "command": "notifications.clearAll"
  },
  {
    "key": "cmd+alt+v",
    "command": "workbench.debug.action.focusVariablesView"
  }
]
JSON

glade_vsix="$(find "$ROOT/contrib/vscode-glade/dist" -name 'vscode-glade-*.vsix' -print -quit)"
if [[ -z "$glade_vsix" ]]; then
  echo "missing Glade VSIX" >&2
  exit 1
fi

code --user-data-dir "$VSCODE_USER" --extensions-dir "$VSCODE_EXTENSIONS" --install-extension "$glade_vsix"
code --user-data-dir "$VSCODE_USER" --extensions-dir "$VSCODE_EXTENSIONS" --install-extension "$CATPPUCCIN_EXTENSION_ID"
code --user-data-dir "$VSCODE_USER" --extensions-dir "$VSCODE_EXTENSIONS" --install-extension "$SALESFORCE_APEX_EXTENSION_ID"

if [[ "${INSTALL_SALESFORCE_EXTENSIONS:-0}" == "1" ]]; then
  code --user-data-dir "$VSCODE_USER" --extensions-dir "$VSCODE_EXTENSIONS" --install-extension salesforce.salesforcedx-vscode
fi

echo "Installed extensions in capture profile:"
# This is the scoped `code --list-extensions` check for the clean capture profile.
code --user-data-dir "$VSCODE_USER" --extensions-dir "$VSCODE_EXTENSIONS" --list-extensions

echo
echo "Prepared clean VS Code profile at ${VSCODE_WINDOW_ZOOM}x UI zoom with Catppuccin Mocha."
echo "Only Glade, Catppuccin, Salesforce Apex, its Salesforce dependencies, and optional Salesforce pack extensions should appear."

vscode_open_command=(
  env
  HOME="$SF_CAPTURE_HOME"
  SF_USE_GENERIC_UNIX_KEYCHAIN=true
  SFDX_USE_GENERIC_UNIX_KEYCHAIN=true
  SF_DISABLE_TELEMETRY=true
  SFDX_DISABLE_TELEMETRY=true
  SF_SKIP_NEW_VERSION_CHECK=true
  code
  --user-data-dir "$VSCODE_USER"
  --extensions-dir "$VSCODE_EXTENSIONS"
  --password-store=basic
  --use-mock-keychain
  --skip-welcome
  --disable-extension github.copilot
  --disable-extension github.copilot-chat
  --disable-extension TypeScriptTeam.jsts-chat-features
  --disable-extension vscode.git
  --disable-extension vscode.github
  --disable-extension vscode.github-authentication
  --new-window "$PROJECT_ROOT"
  "$VSCODE_OPEN_FILE"
)

if [[ "${OPEN_HELP_CAPTURE_APPS:-0}" == "1" ]]; then
  echo
  echo "Opening clean VS Code profile."
  # Keep the macOS VS Code profile path short. Code puts IPC sockets under it.
  # Open a real Apex file first so a bare profile does not show the Copilot welcome sheet.
  "${vscode_open_command[@]}"

  echo
  echo "Opening fresh terminal window for CLI screenshots."
  if [[ "$(uname -s)" == "Darwin" ]]; then
    open -na Ghostty.app --args \
      --working-directory="$PROJECT_ROOT" \
      --title="Glade Help CLI Capture" \
      --font-size="$GHOSTTY_FONT_SIZE" \
      --window-width="$GHOSTTY_WINDOW_WIDTH" \
      --window-height="$GHOSTTY_WINDOW_HEIGHT" \
      --window-show-tab-bar=never
  elif command -v ghostty >/dev/null 2>&1; then
    ghostty \
      --working-directory="$PROJECT_ROOT" \
      --title="Glade Help CLI Capture" \
      --font-size="$GHOSTTY_FONT_SIZE" \
      --window-width="$GHOSTTY_WINDOW_WIDTH" \
      --window-height="$GHOSTTY_WINDOW_HEIGHT" \
      --window-show-tab-bar=never >/dev/null 2>&1 &
  else
    echo "Preferred terminal was not found. Open a fresh terminal window in $PROJECT_ROOT before capturing CLI screenshots." >&2
  fi
fi

cat <<EOF

Capture checklist:
  1. The script prepares the project and clean VS Code profile first.
  2. It does not open apps unless OPEN_HELP_CAPTURE_APPS=1 is set.
  3. Use VS Code for /help screenshots that need editor UI.
  4. Use a terminal for CLI screenshots.
  5. Start each CLI screenshot in a fresh terminal window with no tabs.
  6. Close welcome sheets, side chats, extra panels, and unrelated tabs before capture.
  7. Do not disable Salesforce Core or Services. The Apex grammar depends on them.
  8. Keep screenshots between 900x500 and 2200x1500 pixels.
     VS Code screenshots should use -R80,80,$VSCODE_CAPTURE_WIDTH,$VSCODE_CAPTURE_HEIGHT.
  9. For sf target screenshots, use this disposable CLI home:
     export GLADE_SF_HOME="$SF_CAPTURE_HOME"
     mkdir -p "\$GLADE_SF_HOME"
     export HOME="\$GLADE_SF_HOME"
     export SF_USE_GENERIC_UNIX_KEYCHAIN=true
     export SFDX_USE_GENERIC_UNIX_KEYCHAIN=true
     export SF_DISABLE_TELEMETRY=true
     export SFDX_DISABLE_TELEMETRY=true
     export SF_SKIP_NEW_VERSION_CHECK=true
  10. Save each screenshot with screencapture into:
     $SCREENSHOT_DIR

Open VS Code when ready:
  ${vscode_open_command[*]}

Open terminal when ready:
  open -na Ghostty.app --args --working-directory="$PROJECT_ROOT" --title="Glade Help CLI Capture" --font-size="$GHOSTTY_FONT_SIZE" --window-width="$GHOSTTY_WINDOW_WIDTH" --window-height="$GHOSTTY_WINDOW_HEIGHT" --window-show-tab-bar=never

Example:
  screencapture -x -R80,80,$VSCODE_CAPTURE_WIDTH,$VSCODE_CAPTURE_HEIGHT "$SCREENSHOT_DIR/debug-apex-vscode-01-breakpoint.png"

Run after capture:
  npm --prefix site run help:check
EOF
