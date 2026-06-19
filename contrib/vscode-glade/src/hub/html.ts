import { buildHubHome, buildHubState, type HubAction, type HubSnapshot, type HubStateSection, type HubTaskGroup } from "./model";

export interface HubHtmlOptions {
  cspSource: string;
  nonce: string;
  initialTab?: "home" | "state";
}

export function renderHubHtml(snapshot: HubSnapshot, options: HubHtmlOptions): string {
  const home = buildHubHome(snapshot);
  const state = buildHubState(snapshot);
  const initialTab = options.initialTab === "state" ? "state" : "home";
  const cspSource = escapeAttr(options.cspSource);
  const nonce = escapeAttr(options.nonce);

  return `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <meta http-equiv="Content-Security-Policy" content="default-src 'none'; img-src ${cspSource} https: data:; style-src ${cspSource} 'nonce-${nonce}'; script-src 'nonce-${nonce}';">
  <title>Glade Home</title>
  <style nonce="${nonce}">
    :root {
      color-scheme: light dark;
      --surface: var(--vscode-editor-background);
      --panel: var(--vscode-sideBar-background);
      --panel-strong: var(--vscode-panel-background);
      --text: var(--vscode-editor-foreground);
      --muted: var(--vscode-descriptionForeground);
      --border: var(--vscode-panel-border);
      --focus: var(--vscode-focusBorder);
      --button: var(--vscode-button-background);
      --buttonText: var(--vscode-button-foreground);
      --buttonAlt: var(--vscode-button-secondaryBackground);
      --ok: var(--vscode-testing-iconPassed, #3fb950);
      --warn: var(--vscode-testing-iconQueued, #d29922);
      --error: var(--vscode-testing-iconFailed, #f85149);
    }

    * {
      box-sizing: border-box;
      letter-spacing: 0;
    }

    body {
      margin: 0;
      background: var(--surface);
      color: var(--text);
      font-family: var(--vscode-font-family);
      font-size: var(--vscode-font-size);
    }

    button {
      min-height: 28px;
      border: 1px solid transparent;
      border-radius: 5px;
      padding: 0 10px;
      background: var(--buttonAlt);
      color: var(--text);
      font: inherit;
      cursor: pointer;
      white-space: nowrap;
    }

    button.primary {
      background: var(--button);
      color: var(--buttonText);
    }

    button:focus {
      outline: 1px solid var(--focus);
      outline-offset: 1px;
    }

    .hub {
      min-height: 100vh;
      display: grid;
      grid-template-rows: auto auto 1fr;
    }

    header {
      padding: 14px 18px 10px;
      border-bottom: 1px solid var(--border);
      background: var(--panel-strong);
    }

    h1 {
      margin: 0;
      font-size: 18px;
      font-weight: 650;
      line-height: 1.25;
    }

    .subtitle {
      margin-top: 4px;
      color: var(--muted);
      font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
      font-size: 12px;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    .tabs {
      display: flex;
      gap: 4px;
      padding: 8px 12px 0;
      border-bottom: 1px solid var(--border);
      background: var(--panel);
    }

    .tab {
      border-color: transparent;
      border-bottom-left-radius: 0;
      border-bottom-right-radius: 0;
      background: transparent;
      color: var(--muted);
    }

    .tab[aria-selected="true"] {
      background: var(--surface);
      color: var(--text);
      border-color: var(--border);
      border-bottom-color: var(--surface);
    }

    main {
      min-height: 0;
      padding: 16px;
      overflow: auto;
    }

    .panel[hidden] {
      display: none;
    }

    .grid {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(230px, 1fr));
      gap: 12px;
    }

    .card {
      min-width: 0;
      border: 1px solid var(--border);
      border-radius: 7px;
      background: var(--panel-strong);
      padding: 12px;
    }

    .card-head {
      display: grid;
      grid-template-columns: minmax(0, 1fr) auto;
      gap: 10px;
      align-items: start;
    }

    h2 {
      margin: 0;
      font-size: 14px;
      font-weight: 650;
      line-height: 1.3;
    }

    .summary,
    .detail {
      color: var(--muted);
      line-height: 1.35;
    }

    .summary {
      margin: 6px 0 0;
    }

    .status {
      display: inline-flex;
      max-width: 150px;
      border: 1px solid var(--border);
      border-radius: 999px;
      padding: 2px 8px;
      font-size: 11px;
      line-height: 1.45;
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
    }

    .tone-ok {
      color: var(--ok);
    }

    .tone-warn {
      color: var(--warn);
    }

    .tone-error {
      color: var(--error);
    }

    .tone-muted {
      color: var(--muted);
    }

    .actions {
      display: flex;
      flex-wrap: wrap;
      gap: 7px;
      margin-top: 12px;
    }

    .rows {
      display: grid;
      gap: 8px;
      margin-top: 12px;
    }

    .row {
      display: grid;
      grid-template-columns: 108px minmax(0, 1fr);
      gap: 10px;
      align-items: baseline;
      border-top: 1px solid var(--border);
      padding-top: 8px;
    }

    .label {
      color: var(--muted);
      font-size: 12px;
    }

    .value {
      min-width: 0;
      overflow-wrap: anywhere;
      font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
      font-size: 12px;
    }
  </style>
</head>
<body>
  <div class="hub">
    <header>
      <h1>Glade Home</h1>
      <div class="subtitle">${escapeHtml(snapshot.project?.projectRoot || snapshot.project?.workspaceFolder || "No project open")}</div>
    </header>
    <nav class="tabs" aria-label="Glade Home tabs">
      ${renderTab("home", "Home", initialTab)}
      ${renderTab("state", "State", initialTab)}
    </nav>
    <main>
      <section class="panel" data-panel="home"${initialTab === "home" ? "" : " hidden"}>
        <div class="grid">
          ${home.map(renderTaskGroup).join("")}
        </div>
      </section>
      <section class="panel" data-panel="state"${initialTab === "state" ? "" : " hidden"}>
        <div class="grid">
          ${state.map(renderStateSection).join("")}
        </div>
      </section>
    </main>
  </div>
  <script nonce="${nonce}">
    (function () {
      const vscode = acquireVsCodeApi();

      function selectTab(tab) {
        document.querySelectorAll("[data-tab]").forEach((button) => {
          const selected = button.dataset.tab === tab;
          button.setAttribute("aria-selected", String(selected));
        });
        document.querySelectorAll("[data-panel]").forEach((panel) => {
          panel.hidden = panel.dataset.panel !== tab;
        });
        vscode.postMessage({ type: "selectTab", tab });
      }

      document.addEventListener("click", (event) => {
        const target = event.target;
        if (!(target instanceof Element)) {
          return;
        }
        const tabButton = target.closest("[data-tab]");
        if (tabButton) {
          selectTab(tabButton.dataset.tab);
          return;
        }
        const commandButton = target.closest("[data-command]");
        if (commandButton) {
          vscode.postMessage({ type: "runCommand", command: commandButton.dataset.command });
        }
      });

      vscode.postMessage({ type: "ready" });
    })();
  </script>
</body>
</html>`;
}

function renderTab(tab: "home" | "state", label: string, initialTab: "home" | "state"): string {
  const selected = tab === initialTab;
  return `<button class="tab" type="button" data-tab="${tab}" aria-selected="${selected}">${escapeHtml(label)}</button>`;
}

function renderTaskGroup(group: HubTaskGroup): string {
  const actions = [group.primary, ...group.actions.filter((action) => action.id !== group.primary.id)];
  return `<article class="card" data-task="${escapeAttr(group.id)}">
    <div class="card-head">
      <div>
        <h2>${escapeHtml(group.title)}</h2>
        <p class="summary">${escapeHtml(group.summary)}</p>
      </div>
      ${renderStatus(group.status.label, group.status.tone, group.status.detail)}
    </div>
    <div class="actions">
      ${actions.map((action) => renderAction(action, action.primary || action.id === group.primary.id)).join("")}
    </div>
  </article>`;
}

function renderAction(action: HubAction, primary: boolean): string {
  const title = action.description || action.disabledReason || action.label;
  return `<button type="button" class="${primary ? "primary" : "secondary"}" data-command="${escapeAttr(action.command)}" title="${escapeAttr(title)}"${action.disabledReason ? " disabled" : ""}>${escapeHtml(action.label)}</button>`;
}

function renderStateSection(section: HubStateSection): string {
  return `<article class="card" data-state="${escapeAttr(section.id)}">
    <div class="card-head">
      <h2>${escapeHtml(section.title)}</h2>
      <span class="status tone-${escapeAttr(section.tone)}">${escapeHtml(section.tone)}</span>
    </div>
    <div class="rows">
      ${section.rows.map(renderStateRow).join("")}
    </div>
  </article>`;
}

function renderStateRow(row: HubStateSection["rows"][number]): string {
  return `<div class="row">
    <div class="label">${escapeHtml(row.label)}</div>
    <div>
      <div class="value">${escapeHtml(row.value)}</div>
      ${row.detail ? `<div class="detail">${escapeHtml(row.detail)}</div>` : ""}
    </div>
  </div>`;
}

function renderStatus(label: string, tone: string, detail?: string): string {
  const title = detail ? `${label}: ${detail}` : label;
  return `<span class="status tone-${escapeAttr(tone)}" title="${escapeAttr(title)}">${escapeHtml(label)}</span>`;
}

function escapeHtml(value: string): string {
  return value.replace(/[&<>"']/g, (char) => htmlEscapes[char]);
}

function escapeAttr(value: string): string {
  return escapeHtml(value);
}

const htmlEscapes: Record<string, string> = {
  "&": "&amp;",
  "<": "&lt;",
  ">": "&gt;",
  '"': "&quot;",
  "'": "&#39;",
};
