import { buildHubHome, buildHubState, type HubAction, type HubSnapshot, type HubStateSection, type HubTaskGroup } from "./model";

export interface HubHtmlOptions {
  cspSource: string;
  nonce: string;
  initialTab?: "home" | "state";
  logoUri?: string;
}

export function renderHubHtml(snapshot: HubSnapshot, options: HubHtmlOptions): string {
  const home = buildHubHome(snapshot);
  const state = buildHubState(snapshot);
  const initialTab = options.initialTab === "state" ? "state" : "home";
  const cspSource = escapeAttr(options.cspSource);
  const nonce = escapeAttr(options.nonce);
  const logoUri = options.logoUri ? escapeAttr(options.logoUri) : undefined;

  return `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <meta http-equiv="Content-Security-Policy" content="default-src 'none'; img-src ${cspSource}; style-src ${cspSource} 'nonce-${nonce}'; script-src 'nonce-${nonce}';">
  <title>Glade Home</title>
  <style nonce="${nonce}">
    :root {
      color-scheme: light dark;
      --brand-shell: #070b0d;
      --brand-shell-soft: #0b1215;
      --brand-surface: #10191e;
      --brand-panel: #14232a;
      --brand-panel-active: #1b2f38;
      --brand-line: #26363d;
      --brand-line-strong: #38505a;
      --brand-text: #f3f7f5;
      --brand-muted: #a9b8ad;
      --brand-subtle: #7f9187;
      --brand-inverse: #061009;
      --glade: #9be870;
      --glade-strong: #b7ff8a;
      --glade-muted: rgba(155, 232, 112, 0.14);
      --glade-line: rgba(155, 232, 112, 0.42);
      --surface: var(--brand-shell);
      --panel: var(--brand-shell-soft);
      --panel-strong: var(--brand-surface);
      --text: var(--brand-text);
      --muted: var(--brand-muted);
      --border: var(--brand-line);
      --focus: var(--glade-strong);
      --button: var(--glade);
      --buttonText: var(--brand-inverse);
      --buttonAlt: rgba(255, 255, 255, 0.055);
      --ok: #9be870;
      --warn: #f5c95f;
      --error: #ff6b61;
      --info: #7db7ff;
    }

    * {
      box-sizing: border-box;
      letter-spacing: 0;
    }

    body {
      margin: 0;
      background:
        linear-gradient(180deg, rgba(20, 35, 42, 0.88) 0%, rgba(7, 11, 13, 0) 320px),
        var(--surface);
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
      border-color: var(--glade-strong);
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
      display: flex;
      gap: 12px;
      align-items: center;
      padding: 14px 18px 12px;
      border-bottom: 1px solid var(--border);
      background: rgba(16, 25, 30, 0.88);
    }

    .brand-mark {
      width: 38px;
      height: 38px;
      flex: 0 0 auto;
      border-radius: 9px;
      box-shadow: 0 0 0 1px rgba(155, 232, 112, 0.20);
    }

    .brand-copy {
      min-width: 0;
    }

    .eyebrow {
      margin-bottom: 2px;
      color: var(--glade-strong);
      font-size: 11px;
      font-weight: 650;
      line-height: 1.2;
      text-transform: uppercase;
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
      background: rgba(11, 18, 21, 0.92);
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
      border-color: var(--glade-line);
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

    .grid,
    .card-grid {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(230px, 1fr));
      gap: 12px;
    }

    .home-stack {
      display: grid;
      gap: 18px;
    }

    .home-section {
      min-width: 0;
    }

    .section-head {
      display: flex;
      align-items: baseline;
      justify-content: space-between;
      gap: 12px;
      margin: 0 0 8px;
    }

    .section-title {
      margin: 0;
      color: var(--glade-strong);
      font-size: 11px;
      font-weight: 700;
      line-height: 1.3;
      text-transform: uppercase;
    }

    .section-note {
      color: var(--muted);
      font-size: 12px;
      line-height: 1.35;
      text-align: right;
    }

    .card {
      min-width: 0;
      border: 1px solid var(--border);
      border-radius: 7px;
      background: rgba(16, 25, 30, 0.92);
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
      color: var(--brand-subtle);
    }

    .primary-action {
      margin-top: 12px;
    }

    .primary-action button {
      width: 100%;
      min-height: 34px;
    }

    .more-actions {
      margin-top: 8px;
      border-top: 1px solid var(--border);
      padding-top: 8px;
    }

    .more-actions summary {
      width: max-content;
      border-radius: 5px;
      color: var(--muted);
      cursor: pointer;
      font-size: 12px;
      line-height: 1.5;
      list-style: none;
    }

    .more-actions summary::-webkit-details-marker {
      display: none;
    }

    .more-actions summary:focus {
      outline: 1px solid var(--focus);
      outline-offset: 2px;
    }

    .more-list {
      display: flex;
      flex-wrap: wrap;
      gap: 7px;
      margin-top: 8px;
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
      ${logoUri ? `<img class="brand-mark" src="${logoUri}" alt="Glade">` : ""}
      <div class="brand-copy">
        <div class="eyebrow">Local Apex workbench</div>
        <h1>Glade Home</h1>
        <div class="subtitle">${escapeHtml(snapshot.project?.projectRoot || snapshot.project?.workspaceFolder || "No project open")}</div>
      </div>
    </header>
    <nav class="tabs" aria-label="Glade Home tabs">
      ${renderTab("home", "Home", initialTab)}
      ${renderTab("state", "State", initialTab)}
    </nav>
    <main>
      <section class="panel" data-panel="home"${initialTab === "home" ? "" : " hidden"}>
        ${renderHomePanel(home)}
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
  return `<article class="card" data-task="${escapeAttr(group.id)}">
    <div class="card-head">
      <div>
        <h2>${escapeHtml(group.title)}</h2>
        <p class="summary">${escapeHtml(group.summary)}</p>
      </div>
      ${renderStatus(group.status.label, group.status.tone, group.status.detail)}
    </div>
    <div class="primary-action">
      ${renderAction(group.primary, true)}
    </div>
    ${renderMoreActions(group)}
  </article>`;
}

function renderHomePanel(groups: HubTaskGroup[]): string {
  return `<div class="home-stack">
    ${phaseOrder.map((phase) => {
      const phaseGroups = groups.filter((group) => group.phase === phase);
      if (phaseGroups.length === 0) {
        return "";
      }
      return `<section class="home-section" data-phase="${phase}">
        <div class="section-head">
          <h2 class="section-title">${escapeHtml(phaseTitle(phase))}</h2>
          <div class="section-note">${escapeHtml(phaseNote(phase))}</div>
        </div>
        <div class="card-grid">
          ${phaseGroups.map(renderTaskGroup).join("")}
        </div>
      </section>`;
    }).join("")}
  </div>`;
}

function renderMoreActions(group: HubTaskGroup): string {
  const actions = group.actions.filter((action) => action.id !== group.primary.id);
  if (actions.length === 0) {
    return "";
  }
  return `<details class="more-actions">
    <summary>More</summary>
    <div class="more-list">
      ${actions.map((action) => renderAction(action, false)).join("")}
    </div>
  </details>`;
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

const phaseOrder = ["setup", "daily", "data"] as const;

function phaseTitle(phase: typeof phaseOrder[number]): string {
  switch (phase) {
    case "setup":
      return "Set up";
    case "daily":
      return "Day to day";
    case "data":
      return "Database";
  }
}

function phaseNote(phase: typeof phaseOrder[number]): string {
  switch (phase) {
    case "setup":
      return "connect the project";
    case "daily":
      return "tests and scratch";
    case "data":
      return "inspect and switch";
  }
}
