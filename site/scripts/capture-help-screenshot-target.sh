#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SITE="$ROOT/site"
PROJECT_ROOT="${HELP_PROJECT_ROOT:-/tmp/glade-help-capture/macrodata-apex}"
SCREENSHOT_DIR="$SITE/docs-src/public/help/screenshots"
CAPTURE_ROOT="$ROOT/.glade/help-capture"
CAPTURE_XDG_DATA_HOME="${HELP_SCREENSHOT_XDG_DATA_HOME:-/tmp/glade-help-capture/share}"
VSCODE_PROFILE_ROOT="${TMPDIR:-/tmp}/glade-help-vscode"
VSCODE_USER="$VSCODE_PROFILE_ROOT/user"
VSCODE_EXTENSIONS="$VSCODE_PROFILE_ROOT/extensions"
SF_CAPTURE_HOME="$PROJECT_ROOT/.glade/sf-home"
PROMPT='macrodata-apex % '

TERMINAL_RECT="${TERMINAL_RECT:-80,80,954,472}"
TERMINAL_WIDE_RECT="${TERMINAL_WIDE_RECT:-80,80,1100,472}"
TERMINAL_SETTLE_SECONDS="${TERMINAL_SETTLE_SECONDS:-2}"
VSCODE_RECT="${VSCODE_RECT:-80,80,1100,750}"

targets=(
  first-local-check-01-doctor
  first-local-check-02-check
  run-one-apex-test-01-cli
  run-one-apex-test-02-codelens
  run-one-apex-test-03-test-explorer
  debug-apex-vscode-01-breakpoint
  debug-apex-vscode-02-debug-toolbar
  debug-apex-vscode-03-variables
  anonymous-apex-scratch-01-buffer
  anonymous-apex-scratch-02-run
  local-data-environments-01-sidebar
  local-data-environments-02-terminal
  changed-tests-before-pr-01-changed-tests
  changed-tests-before-pr-02-reports
  glade-org-sf-data-import-01-create-start
  glade-org-sf-data-import-02-auth-list
  glade-org-sf-data-import-03-import-query
  profile-apex-debug-log-01-profile
  profile-apex-debug-log-02-json
  ci-setup-01-workflow
  ci-setup-02-artifacts
)

usage() {
  cat <<EOF
Usage:
  npm --prefix site run help:screenshot -- --list
  npm --prefix site run help:screenshot -- --all
  npm --prefix site run help:screenshot -- <target>

Targets:
$(printf '  %s\n' "${targets[@]}")
EOF
}

need_command() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "missing required command: $1" >&2
    exit 1
  fi
}

require_macos() {
  if [[ "$(uname -s)" != "Darwin" ]]; then
    echo "help screenshot automation uses macOS app capture tools" >&2
    exit 1
  fi
}

rect_parts() {
  local rect="$1"
  IFS=, read -r rect_x rect_y rect_w rect_h <<<"$rect"
}

position_app_window() {
  local _app_name="$1"
  local process_name="$2"
  local rect="$3"
  rect_parts "$rect"
  osascript >/dev/null <<APPLESCRIPT
tell application "System Events"
  repeat with attempt from 1 to 80
    if exists process "$process_name" then
      tell process "$process_name"
        set frontmost to true
        if exists window 1 then
          set position of window 1 to {$rect_x, $rect_y}
          set size of window 1 to {$rect_w, $rect_h}
          return ""
        end if
      end tell
    end if
    delay 0.25
  end repeat
end tell
error "could not find window for process $process_name"
APPLESCRIPT
}

position_process_window() {
  local process_name="$1"
  local pid="$2"
  local rect="$3"
  rect_parts "$rect"
  osascript <<APPLESCRIPT
tell application "System Events"
  repeat with p in (every process whose name is "$process_name")
    if (unix id of p as integer) is not $pid then
      if exists window 1 of p then
        set position of window 1 of p to {1400, 80}
      end if
    end if
  end repeat
  repeat with p in (every process whose unix id is $pid)
    set frontmost of p to true
    if exists window 1 of p then
      set position of window 1 of p to {$rect_x, $rect_y}
      set size of window 1 of p to {$rect_w, $rect_h}
    end if
  end repeat
end tell
APPLESCRIPT
}

capture_rect() {
  local target="$1"
  local rect="$2"
  mkdir -p "$SCREENSHOT_DIR"
  screencapture -x -R"$rect" "$SCREENSHOT_DIR/$target.png"
  identify -format '%f %wx%h\n' "$SCREENSHOT_DIR/$target.png" 2>/dev/null || true
}

ensure_project() {
  local project_path
  project_path="$(HELP_PROJECT_ROOT="$PROJECT_ROOT" npm --prefix "$SITE" run --silent help:project)"
  if [[ "$project_path" != "$PROJECT_ROOT" ]]; then
    echo "unexpected project path: $project_path" >&2
    exit 1
  fi
}

mark_changed_apex_file() {
  printf '\n// Local edit for changed-test walkthrough.\n' >> \
    "$PROJECT_ROOT/force-app/main/default/classes/RefinementService.cls"
}

prepare_capture_toolchain() {
  local source_path=""
  source_path="$(glade toolchain status --json 2>/dev/null | sed -n 's/^[[:space:]]*"path": "\(.*\)",[[:space:]]*$/\1/p' | head -1)"
  if [[ -n "$source_path" && -d "$source_path" ]]; then
    mkdir -p "$CAPTURE_XDG_DATA_HOME"
    rm -rf "$CAPTURE_XDG_DATA_HOME/glade"
    ln -s "$source_path" "$CAPTURE_XDG_DATA_HOME/glade"
  fi
}

ensure_vscode_profile() {
  if [[ ! -f "$VSCODE_USER/User/settings.json" ]] ||
    ! grep -q '"git.enabled": false' "$VSCODE_USER/User/settings.json" ||
    ! grep -q '"workbench.activityBar.location": "hidden"' "$VSCODE_USER/User/settings.json" ||
    ! grep -q '"debug.autoExpandLazyVariables": "on"' "$VSCODE_USER/User/settings.json"; then
    HELP_PROJECT_ROOT="$PROJECT_ROOT" bash "$SITE/scripts/capture-help-screenshots.sh" >/dev/null
  fi
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
}

terminal_done_file() {
  local target="$1"
  local done_dir="$CAPTURE_ROOT/terminal-done"
  mkdir -p "$done_dir"
  printf '%s/%s-%s-%s.done\n' "$done_dir" "$target" "$$" "$(date +%s)"
}

terminal_ready_file() {
  local target="$1"
  local ready_dir="$CAPTURE_ROOT/terminal-ready"
  mkdir -p "$ready_dir"
  printf '%s/%s-%s-%s.ready\n' "$ready_dir" "$target" "$$" "$(date +%s)"
}

terminal_command_file() {
  local target="$1"
  local body="$2"
  local done_file="$3"
  local ready_file="$4"
  local script_dir="$CAPTURE_ROOT/terminal-scripts"
  local script_file="$script_dir/$target-$$-$(date +%s).zsh"
  mkdir -p "$script_dir"
  rm -f "$done_file"
  rm -f "$ready_file"
  {
    printf '#!/bin/zsh -f\n'
    printf 'cd %q\n' "$PROJECT_ROOT"
    printf 'export XDG_DATA_HOME=%q\n' "$CAPTURE_XDG_DATA_HOME"
    printf 'PROMPT=%q\n' "$PROMPT"
    printf 'printf "\\033c"\n'
    printf '%s\n' "$body"
    printf 'printf "%%s" "$PROMPT"\n'
    printf 'touch %q\n' "$ready_file"
    printf 'while [[ ! -f %q ]]; do sleep 0.2; done\n' "$done_file"
  } > "$script_file"
  chmod +x "$script_file"
  printf '%s\n' "$script_file"
}

find_ghostty_pid_for_command() {
  local command_file="$1"
  ps -axo pid=,command= | awk -v file="$command_file" 'index($0, file) { print $1; exit }'
}

wait_for_ghostty_pid_for_command() {
  local command_file="$1"
  local pid=""
  for ((i = 0; i < 40; i += 1)); do
    pid="$(find_ghostty_pid_for_command "$command_file")"
    if [[ -n "$pid" ]]; then
      printf '%s\n' "$pid"
      return 0
    fi
    sleep 0.25
  done
  echo "could not find terminal process for $command_file" >&2
  return 1
}

wait_for_terminal_exit() {
  local command_file="$1"
  local done_file="$2"
  touch "$done_file"
  for ((i = 0; i < 40; i += 1)); do
    if ! pgrep -f "$command_file" >/dev/null 2>&1; then
      rm -f "$done_file"
      return 0
    fi
    sleep 0.25
  done
  pkill -f "$command_file" >/dev/null 2>&1 || true
  rm -f "$done_file"
}

wait_for_terminal_ready() {
  local command_file="$1"
  local ready_file="$2"
  for ((i = 0; i < 160; i += 1)); do
    if [[ -f "$ready_file" ]]; then
      return 0
    fi
    if ! pgrep -f "$command_file" >/dev/null 2>&1; then
      echo "terminal process exited before capture was ready: $command_file" >&2
      return 1
    fi
    sleep 0.25
  done
  echo "terminal capture did not become ready: $command_file" >&2
  return 1
}

run_terminal() {
  local target="$1"
  local rect="$2"
  local body="$3"
  local fixture_action="${4:-}"
  local capture_mode="${5:-ready}"
  local should_ensure_project=1
  require_macos
  need_command open
  need_command osascript
  need_command screencapture

  case "$fixture_action" in
    "")
      ;;
    changed-apex)
      ;;
    reuse-project)
      should_ensure_project=0
      ;;
    *)
      echo "unknown fixture action: $fixture_action" >&2
      exit 1
      ;;
  esac
  if [[ "$should_ensure_project" == 1 ]]; then
    ensure_project
  fi
  if [[ "$fixture_action" == "changed-apex" ]]; then
    mark_changed_apex_file
  fi
  prepare_capture_toolchain
  local done_file
  done_file="$(terminal_done_file "$target")"
  local ready_file
  ready_file="$(terminal_ready_file "$target")"
  local command_file
  command_file="$(terminal_command_file "$target" "$body" "$done_file" "$ready_file")"
  open -na Ghostty.app --args \
    --working-directory="$PROJECT_ROOT" \
    --title="Glade Help CLI Capture" \
    --font-size=18 \
    --window-width=96 \
    --window-height=28 \
    --window-show-tab-bar=never \
    --command="$command_file"
  local terminal_pid
  terminal_pid="$(wait_for_ghostty_pid_for_command "$command_file")"
  sleep 2
  position_process_window "Ghostty" "$terminal_pid" "$rect"
  sleep "$TERMINAL_SETTLE_SECONDS"
  if [[ "$capture_mode" == "ready" ]]; then
    wait_for_terminal_ready "$command_file" "$ready_file"
    sleep 0.5
  else
    sleep 1
  fi
  capture_rect "$target" "$rect"
  wait_for_terminal_exit "$command_file" "$done_file"
  rm -f "$ready_file"
}

run_terminal_with_server() {
  ensure_project
  rm -rf "$PROJECT_ROOT/.glade/orgs/refine-local"* "$PROJECT_ROOT/.glade/sf-home"
  run_terminal glade-org-sf-data-import-01-create-start "$TERMINAL_RECT" $'printf "%sglade org create refine-local --project .\\n" "$PROMPT"\nglade org create refine-local --project .\nprintf "%sglade org start refine-local --project .\\n" "$PROMPT"\nglade org start refine-local --project .' "" live
}

ensure_local_org_server() {
  ensure_project
  local org_file="$PROJECT_ROOT/.glade/orgs/refine-local/org.json"
  if [[ ! -f "$org_file" ]] || ! pgrep -f "glade org start refine-local --project $PROJECT_ROOT" >/dev/null 2>&1; then
    pkill -f "glade org start refine-local --project $PROJECT_ROOT" >/dev/null 2>&1 || true
    glade org create refine-local --project "$PROJECT_ROOT" >/dev/null
    nohup glade org start refine-local --project "$PROJECT_ROOT" >/tmp/glade-help-refine-local.log 2>&1 &
    sleep 3
  fi
}

stop_local_org_server() {
  pkill -f "glade org start refine-local --project $PROJECT_ROOT" >/dev/null 2>&1 || true
}

close_vscode_capture_windows() {
  pkill -f "$VSCODE_USER" >/dev/null 2>&1 || true
  pkill -f "$VSCODE_PROFILE_ROOT" >/dev/null 2>&1 || true
  sleep 0.5
  pkill -9 -f "$VSCODE_USER" >/dev/null 2>&1 || true
  pkill -9 -f "$VSCODE_PROFILE_ROOT" >/dev/null 2>&1 || true
  sleep 0.2
}

cleanup_help_screenshot_capture() {
  stop_local_org_server
  close_vscode_capture_windows
}

open_vscode_file() {
  local file="$1"
  require_macos
  need_command code
  need_command osascript
  need_command screencapture
  need_command cliclick

  ensure_project
  ensure_vscode_profile
  close_vscode_capture_windows
  env \
    HOME="$SF_CAPTURE_HOME" \
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
      --skip-welcome \
      --disable-extension github.copilot \
      --disable-extension github.copilot-chat \
      --disable-extension TypeScriptTeam.jsts-chat-features \
      --disable-extension vscode.git \
      --disable-extension vscode.github \
      --disable-extension vscode.github-authentication \
      --new-window "$PROJECT_ROOT" \
      "$file" >/dev/null 2>&1
  sleep 5
  position_app_window "Visual Studio Code" "Code" "$VSCODE_RECT"
  dismiss_vscode_first_run_prompts
  sleep 1
}

open_vscode_location() {
  local file="$1"
  local line="$2"
  local column="${3:-1}"
  env \
    HOME="$SF_CAPTURE_HOME" \
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
      --skip-welcome \
      --disable-extension github.copilot \
      --disable-extension github.copilot-chat \
      --disable-extension TypeScriptTeam.jsts-chat-features \
      --disable-extension vscode.git \
      --disable-extension vscode.github \
      --disable-extension vscode.github-authentication \
      --reuse-window \
      --goto "$file:$line:$column" >/dev/null 2>&1
  sleep 1
  position_app_window "Visual Studio Code" "Code" "$VSCODE_RECT"
  dismiss_vscode_first_run_prompts
  sleep 0.3
}

stop_debugging_if_active() {
  osascript >/dev/null <<'APPLESCRIPT'
tell application "System Events"
  repeat with attempt from 1 to 40
    if exists process "Code" then
      tell process "Code"
        set frontmost to true
        try
          click menu item "Stop Debugging" of menu "Run" of menu bar item "Run" of menu bar 1
        end try
      end tell
      return ""
    end if
    delay 0.25
  end repeat
end tell
return ""
APPLESCRIPT
}

clear_vscode_breakpoints() {
  osascript >/dev/null <<'APPLESCRIPT'
tell application "System Events"
  repeat with attempt from 1 to 40
    if exists process "Code" then
      tell process "Code"
        set frontmost to true
        try
          click menu item "Remove All Breakpoints" of menu "Run" of menu bar item "Run" of menu bar 1
        end try
      end tell
      return ""
    end if
    delay 0.25
  end repeat
end tell
return ""
APPLESCRIPT
}

set_vscode_breakpoint_from_menu() {
  clear_vscode_breakpoints
  osascript >/dev/null <<'APPLESCRIPT'
tell application "System Events"
  repeat with attempt from 1 to 40
    if exists process "Code" then
      tell process "Code"
        set frontmost to true
        click menu item "Toggle Breakpoint" of menu "Run" of menu bar item "Run" of menu bar 1
      end tell
      return ""
    end if
    delay 0.25
  end repeat
end tell
error "could not find Code process"
APPLESCRIPT
}

start_debugging_from_menu() {
  osascript >/dev/null <<'APPLESCRIPT'
tell application "System Events"
  repeat with attempt from 1 to 40
    if exists process "Code" then
      tell process "Code"
        set frontmost to true
        delay 0.2
        click menu item "Start Debugging" of menu "Run" of menu bar item "Run" of menu bar 1
      end tell
      return ""
    end if
    delay 0.25
  end repeat
end tell
error "could not find Code process"
APPLESCRIPT
}

show_explorer_view() {
  osascript >/dev/null <<'APPLESCRIPT'
tell application "System Events"
  repeat with attempt from 1 to 40
    if exists process "Code" then
      tell process "Code"
        set frontmost to true
        keystroke "e" using {command down, shift down}
      end tell
      return ""
    end if
    delay 0.25
  end repeat
end tell
error "could not find Code process"
APPLESCRIPT
}

show_run_and_debug_view() {
  osascript >/dev/null <<'APPLESCRIPT'
tell application "System Events"
  repeat with attempt from 1 to 40
    if exists process "Code" then
      tell process "Code"
        set frontmost to true
        keystroke "d" using {command down, shift down}
      end tell
      return ""
    end if
    delay 0.25
  end repeat
end tell
error "could not find Code process"
APPLESCRIPT
}

open_glade_sidebar() {
  osascript >/dev/null <<'APPLESCRIPT'
tell application "System Events"
  repeat with attempt from 1 to 40
    if exists process "Code" then
      tell process "Code"
        set frontmost to true
        keystroke "g" using {command down, option down}
        delay 0.5
        keystroke "e" using {command down, option down}
      end tell
      return ""
    end if
    delay 0.25
  end repeat
end tell
error "could not find Code process"
APPLESCRIPT
}

focus_debug_variables_view() {
  osascript >/dev/null <<'APPLESCRIPT'
tell application "System Events"
  repeat with attempt from 1 to 40
    if exists process "Code" then
      tell process "Code"
        set frontmost to true
        keystroke "v" using {command down, option down}
      end tell
      return ""
    end if
    delay 0.25
  end repeat
end tell
error "could not find Code process"
APPLESCRIPT
}

dismiss_vscode_first_run_prompts() {
  osascript >/dev/null <<'APPLESCRIPT'
tell application "System Events"
  repeat with attempt from 1 to 40
    if exists process "Code" then
      tell process "Code"
        set frontmost to true
        repeat 3 times
          key code 53
          delay 0.1
        end repeat
        keystroke "n" using {command down, option down}
        delay 0.2
        key code 53
      end tell
      return ""
    end if
    delay 0.25
  end repeat
end tell
error "could not find Code process"
APPLESCRIPT
}

reset_vscode_capture_state() {
  stop_debugging_if_active >/dev/null 2>&1 || true
  clear_vscode_breakpoints >/dev/null 2>&1 || true
  dismiss_vscode_first_run_prompts >/dev/null 2>&1 || true
  key_code 53 >/dev/null 2>&1 || true
  sleep 0.4
}

click_relative() {
  rect_parts "$VSCODE_RECT"
  cliclick "c:$((rect_x + $1)),$((rect_y + $2))"
}

key_code() {
  local code="$1"
  osascript >/dev/null <<APPLESCRIPT
tell application "System Events"
  repeat with attempt from 1 to 40
    if exists process "Code" then
      tell process "Code"
        set frontmost to true
        key code $code
      end tell
      return ""
    end if
    delay 0.25
  end repeat
end tell
error "could not find Code process"
APPLESCRIPT
}

expand_debug_locals() {
  show_run_and_debug_view
  sleep 0.5
  focus_debug_variables_view
  sleep 0.3
  key_code 124
  sleep 0.2
  key_code 125
  sleep 0.2
  key_code 124
  sleep 0.5
}

close_bottom_panel() {
  click_relative 1070 444
  sleep 0.4
}

write_launch_config() {
  mkdir -p "$PROJECT_ROOT/.vscode"
  cat > "$PROJECT_ROOT/.vscode/launch.json" <<'JSON'
{
  "version": "0.2.0",
  "configurations": [
    {
      "type": "glade",
      "request": "launch",
      "name": "Debug opensFile (macrodata-apex)",
      "project": "${workspaceFolder}",
      "source": "opensFile();",
      "className": "RefinementServiceTest",
      "methodName": "opensFile"
    }
  ]
}
JSON
}

clear_vscode_launch_config() {
  rm -f "$PROJECT_ROOT/.vscode/launch.json"
  rmdir "$PROJECT_ROOT/.vscode" >/dev/null 2>&1 || true
}

write_ci_workflow() {
  mkdir -p "$PROJECT_ROOT/.github/workflows"
  cat > "$PROJECT_ROOT/.github/workflows/glade.yml" <<'YAML'
name: glade
on: [pull_request]

jobs:
  glade:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      - run: curl -fsSL https://glade.sh/install.sh | sh
      - run: echo "$HOME/.local/bin" >> "$GITHUB_PATH"
      - run: glade doctor
      - run: mkdir -p reports
      - run: glade check --project . --format sarif --output reports/glade-check.sarif --no-progress
      - run: glade test changed --project . --since origin/main --json --no-progress > reports/glade-test-changed.json
      - run: glade test --project . --junit reports/glade-junit.xml --no-progress
      - uses: actions/upload-artifact@v4
        if: always()
        with:
          name: glade-results
          path: |
            reports/glade-check.sarif
            reports/glade-test-changed.json
            reports/glade-junit.xml
YAML
}

run_vscode() {
  local target="$1"
  local file="$2"
  local actions="${3:-}"
  case "$actions" in
    debug-toolbar|debug-variables|debug-breakpoint)
      ;;
    *)
      clear_vscode_launch_config
      ;;
  esac
  open_vscode_file "$file"
  reset_vscode_capture_state
  open_vscode_location "$file" 1 1
  case "$actions" in
    none)
      show_explorer_view
      ;;
    testing)
      click_relative 51 390
      sleep 1
      click_relative 73 224
      ;;
    breakpoint)
      open_vscode_location "$file" 4 1
      set_vscode_breakpoint_from_menu
      ;;
    debug-toolbar)
      write_launch_config
      local breakpoint_file="$PROJECT_ROOT/force-app/main/default/classes/RefinementService.cls"
      open_vscode_location "$breakpoint_file" 4 1
      set_vscode_breakpoint_from_menu
      start_debugging_from_menu
      sleep 4
      show_run_and_debug_view
      ;;
    debug-variables)
      write_launch_config
      click_relative 103 242
      sleep 0.3
      key_code 96
      sleep 4
      click_relative 51 296
      ;;
    debug-breakpoint)
      write_launch_config
      local breakpoint_file="$PROJECT_ROOT/force-app/main/default/classes/RefinementService.cls"
      open_vscode_location "$breakpoint_file" 4 1
      set_vscode_breakpoint_from_menu
      start_debugging_from_menu
      sleep "${HELP_SCREENSHOT_DEBUG_WAIT:-8}"
      expand_debug_locals
      ;;
    glade-data)
      open_glade_sidebar
      sleep 1
      ;;
    *)
      echo "unknown VS Code action: $actions" >&2
      exit 1
      ;;
  esac
  sleep 1
  close_bottom_panel
  capture_rect "$target" "$VSCODE_RECT"
  if [[ "$actions" == "debug-toolbar" || "$actions" == "debug-breakpoint" ]]; then
    stop_debugging_if_active >/dev/null 2>&1 || true
  fi
  close_vscode_capture_windows
}

run_target() {
  local target="$1"
  local since_ref="${HELP_SCREENSHOT_SINCE:-HEAD}"

  case "$target" in
    first-local-check-01-doctor)
      run_terminal "$target" "$TERMINAL_RECT" $'printf "%sglade version\\n" "$PROMPT"\nglade version\nprintf "%sglade doctor\\n" "$PROMPT"\nglade doctor'
      ;;
    first-local-check-02-check)
      run_terminal "$target" "$TERMINAL_RECT" $'printf "%stest -f glade.yml || glade init --project . --yes\\n" "$PROMPT"\ntest -f glade.yml || glade init --project . --yes\nprintf "%sglade config validate --project .\\n" "$PROMPT"\nglade config validate --project .\nprintf "%sglade check --project . --no-progress\\n" "$PROMPT"\nglade check --project . --no-progress'
      ;;
    run-one-apex-test-01-cli)
      run_terminal "$target" "$TERMINAL_RECT" $'printf "%sglade test --project . --class RefinementServiceTest --no-progress\\n" "$PROMPT"\nglade test --project . --class RefinementServiceTest --no-progress'
      ;;
    run-one-apex-test-02-codelens)
      run_vscode "$target" "$PROJECT_ROOT/force-app/main/default/classes/RefinementServiceTest.cls" none
      ;;
    run-one-apex-test-03-test-explorer)
      run_vscode "$target" "$PROJECT_ROOT/force-app/main/default/classes/RefinementService.cls" debug-breakpoint
      ;;
    debug-apex-vscode-01-breakpoint)
      run_vscode "$target" "$PROJECT_ROOT/force-app/main/default/classes/RefinementService.cls" breakpoint
      ;;
    debug-apex-vscode-02-debug-toolbar)
      run_vscode "$target" "$PROJECT_ROOT/force-app/main/default/classes/RefinementServiceTest.cls" debug-toolbar
      ;;
    debug-apex-vscode-03-variables)
      run_vscode "$target" "$PROJECT_ROOT/force-app/main/default/classes/RefinementService.cls" debug-breakpoint
      ;;
    anonymous-apex-scratch-01-buffer)
      run_vscode "$target" "$PROJECT_ROOT/anonymous.apex" none
      ;;
    anonymous-apex-scratch-02-run)
      ensure_project
      glade exec --project "$PROJECT_ROOT" --debug-log "$PROJECT_ROOT/reports/anonymous-output.txt" "$(cat "$PROJECT_ROOT/anonymous.apex")" >/dev/null
      run_vscode "$target" "$PROJECT_ROOT/reports/anonymous-output.txt" none
      ;;
    local-data-environments-01-sidebar)
      run_vscode "$target" "$PROJECT_ROOT/force-app/main/default/classes/RefinementServiceTest.cls" glade-data
      ;;
    local-data-environments-02-terminal)
      run_terminal "$target" "$TERMINAL_RECT" $'printf "%smkdir -p .glade/envs\\n" "$PROMPT"\nmkdir -p .glade/envs\nprintf "%sglade db seed --db .glade/envs/dev.sqlite --project . --no-progress seed.json\\n" "$PROMPT"\nglade db seed --db .glade/envs/dev.sqlite --project . --no-progress seed.json\nprintf "%sglade db inspect --db .glade/envs/dev.sqlite --project .\\n" "$PROMPT"\nglade db inspect --db .glade/envs/dev.sqlite --project .'
      ;;
    changed-tests-before-pr-01-changed-tests)
      run_terminal "$target" "$TERMINAL_RECT" "printf \"%sglade test changed --project . --since $since_ref --no-progress\\n\" \"\$PROMPT\"
glade test changed --project . --since $since_ref --no-progress" changed-apex
      ;;
    changed-tests-before-pr-02-reports)
      run_terminal "$target" "$TERMINAL_WIDE_RECT" "printf \"%smkdir -p reports\\n\" \"\$PROMPT\"
mkdir -p reports
printf \"%sglade test changed --project . --since $since_ref --json --no-progress > reports/glade-test-changed.json\\n\" \"\$PROMPT\"
glade test changed --project . --since $since_ref --json --no-progress > reports/glade-test-changed.json
printf \"%sglade test --project . --junit reports/glade-junit.xml --no-progress >/dev/null\\n\" \"\$PROMPT\"
glade test --project . --junit reports/glade-junit.xml --no-progress >/dev/null
printf \"%swc -c reports/glade-test-changed.json reports/glade-junit.xml\\n\" \"\$PROMPT\"
wc -c reports/glade-test-changed.json reports/glade-junit.xml" changed-apex
      ;;
    glade-org-sf-data-import-01-create-start)
      run_terminal_with_server
      ;;
    glade-org-sf-data-import-02-auth-list)
      ensure_local_org_server
      run_terminal "$target" "$TERMINAL_WIDE_RECT" $'printf "%sexport HOME=.glade/sf-home\\n" "$PROMPT"\nexport HOME="$PWD/.glade/sf-home"\nmkdir -p "$HOME"\nexport SF_USE_GENERIC_UNIX_KEYCHAIN=true\nexport SFDX_USE_GENERIC_UNIX_KEYCHAIN=true\nexport SF_DISABLE_TELEMETRY=true\nexport SF_SKIP_NEW_VERSION_CHECK=true\nprintf "%sglade org auth refine-local --project .\\n" "$PROMPT"\nglade org auth refine-local --project .\nprintf "%ssf org list\\n" "$PROMPT"\nsf org list' reuse-project
      ;;
    glade-org-sf-data-import-03-import-query)
      ensure_local_org_server
      run_terminal "$target" "$TERMINAL_WIDE_RECT" $'export HOME="$PWD/.glade/sf-home"\nmkdir -p "$HOME"\nexport SF_USE_GENERIC_UNIX_KEYCHAIN=true\nexport SFDX_USE_GENERIC_UNIX_KEYCHAIN=true\nexport SF_DISABLE_TELEMETRY=true\nexport SF_SKIP_NEW_VERSION_CHECK=true\nglade org auth refine-local --project . >/dev/null\nprintf "%ssf data import tree --plan data/insertOrder.json --target-org refine-local\\n" "$PROMPT"\nsf data import tree --plan data/insertOrder.json --target-org refine-local\nprintf "%ssf data query --query \\"SELECT Id, Name FROM Account\\" --target-org refine-local\\n" "$PROMPT"\nsf data query --query "SELECT Id, Name FROM Account" --target-org refine-local' reuse-project
      ;;
    profile-apex-debug-log-01-profile)
      run_terminal "$target" "$TERMINAL_WIDE_RECT" "printf \"%sglade debug profile --log reports/anonymous-output.txt --format markdown\\n\" \"\$PROMPT\"
profile_md=\"\$(glade debug profile --log reports/anonymous-output.txt --format markdown 2>/dev/null)\"
printf \"%s\\n\" \"\$profile_md\" | awk '
  /^# glade profile/ || /^Events:/ || /^SOQL:/ || /^DML:/ || /^Email:/ || /^CPU:/ { print }
  /^## Hot events/ { hot=1; rows=0; print; next }
  hot && /^[|]/ { print; rows += 1; if (rows >= 4) exit }
'"
      ;;
    profile-apex-debug-log-02-json)
      run_terminal "$target" "$TERMINAL_WIDE_RECT" "printf \"%smkdir -p reports\\n\" \"\$PROMPT\"
mkdir -p reports
printf \"%sexport PROFILE=reports/apex-debug-profile.json\\n\" \"\$PROMPT\"
export PROFILE=reports/apex-debug-profile.json
printf \"%sglade debug profile --log reports/anonymous-output.txt --json > \\\"\$PROFILE\\\"\\n\" \"\$PROMPT\"
glade debug profile --log reports/anonymous-output.txt --json > \"\$PROFILE\"
printf \"%snode -e print-profile-summary\\n\" \"\$PROMPT\"
node -e 'const fs=require(\"fs\"); const p=JSON.parse(fs.readFileSync(process.env.PROFILE,\"utf8\")); console.log(JSON.stringify({status:p.status,events:p.summary.events,dml:p.summary.limits.dml,dmlRows:p.summary.limits.dmlRows,hot:p.data.hot.map((event)=>event.name)}, null, 2));'"
      ;;
    ci-setup-01-workflow)
      ensure_project
      write_ci_workflow
      run_terminal "$target" "$TERMINAL_WIDE_RECT" "printf \"%ssed -n '1,10p;14,17p' .github/workflows/glade.yml\\n\" \"\$PROMPT\"
sed -n '1,10p;14,17p' .github/workflows/glade.yml" reuse-project
      ;;
    ci-setup-02-artifacts)
      run_terminal "$target" "$TERMINAL_WIDE_RECT" "printf \"%smkdir -p reports\\n\" \"\$PROMPT\"
mkdir -p reports
printf \"%sCHECK=reports/glade-check.sarif\\n\" \"\$PROMPT\"
CHECK=reports/glade-check.sarif
printf \"%sCHANGED=reports/glade-test-changed.json\\n\" \"\$PROMPT\"
CHANGED=reports/glade-test-changed.json
printf \"%sJUNIT=reports/glade-junit.xml\\n\" \"\$PROMPT\"
JUNIT=reports/glade-junit.xml
printf \"%sglade check to \$CHECK\\n\" \"\$PROMPT\"
glade check --project . --format sarif --output \"\$CHECK\" --no-progress >/dev/null
printf \"%sglade test changed to \$CHANGED\\n\" \"\$PROMPT\"
glade test changed --project . --since $since_ref --json --no-progress > \"\$CHANGED\"
printf \"%sglade test --junit to \$JUNIT\\n\" \"\$PROMPT\"
glade test --project . --junit \"\$JUNIT\" --no-progress >/dev/null
printf \"%swc -c \\\$CHECK \\\$CHANGED \\\$JUNIT\\n\" \"\$PROMPT\"
wc -c \"\$CHECK\" \"\$CHANGED\" \"\$JUNIT\"" changed-apex
      ;;
    *)
      echo "unknown help screenshot target: $target" >&2
      usage >&2
      exit 1
      ;;
  esac
}

main() {
  local target="${1:-}"
  case "$target" in
    ""|-h|--help)
      usage
      ;;
    --list)
      printf '%s\n' "${targets[@]}"
      ;;
    --all)
      for item in "${targets[@]}"; do
        run_target "$item"
      done
      npm --prefix "$SITE" run help:check
      ;;
    *)
      run_target "$target"
      ;;
  esac
}

trap cleanup_help_screenshot_capture EXIT

main "$@"
