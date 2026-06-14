import {
  InstalledPlugin,
  PluginActionResolutionError,
  PluginActionResolutionResult,
  PluginActionResolutionValues,
  PluginActionView,
  PluginAvailableContexts,
  PluginEditorAction,
  ResolvedPluginAction,
  supportedActionViews,
} from "./model";

const tokenPattern = /\$\{([^}]+)\}/g;

export function actionsForView(
  plugins: InstalledPlugin[],
  view: PluginActionView,
  availableContexts: PluginAvailableContexts = {},
): PluginEditorAction[] {
  if (!supportedActionViews.includes(view)) {
    return [];
  }

  const actions: PluginEditorAction[] = [];
  for (const plugin of plugins) {
    for (const action of plugin.editor?.actions || []) {
      if (!actionVisibleInView(action, view)) {
        continue;
      }
      if (!actionContextsAvailable(action, availableContexts)) {
        continue;
      }
      actions.push(action);
    }
  }
  return actions;
}

export const filterActionsForView = actionsForView;

export function resolveAction(
  action: PluginEditorAction,
  values: PluginActionResolutionValues,
): PluginActionResolutionResult {
  const expanded = expandActionArgs(action.args || [], values);
  if (!expanded.ok) {
    return expanded;
  }

  const resolved: ResolvedPluginAction = {
    id: action.id,
    title: action.title,
    command: action.command,
    args: expanded.args,
    argv: [...action.command, ...expanded.args],
  };
  if (action.icon) {
    resolved.icon = action.icon;
  }
  if (action.output) {
    resolved.output = action.output;
  }
  return { ok: true, action: resolved };
}

export type ExpandedActionArgsResult =
  | { ok: true; args: string[] }
  | { ok: false; error: PluginActionResolutionError };

export function expandActionArgs(args: string[], values: PluginActionResolutionValues): ExpandedActionArgsResult {
  const missingTokens: string[] = [];
  const expandedArgs = args.map((arg) => expandArg(arg, values, missingTokens));
  const uniqueMissingTokens = [...new Set(missingTokens)];
  if (uniqueMissingTokens.length > 0) {
    return { ok: false, error: missingTokenError(uniqueMissingTokens) };
  }
  return { ok: true, args: expandedArgs };
}

function missingTokenError(missingTokens: string[]): PluginActionResolutionError {
  return {
    code: "missingTokenValue",
    message: `Missing values for action tokens: ${missingTokens.join(", ")}`,
    missingTokens,
  };
}

function actionVisibleInView(action: PluginEditorAction, view: PluginActionView): boolean {
  if (!action.view) {
    return true;
  }
  return action.view === view;
}

function actionContextsAvailable(action: PluginEditorAction, availableContexts: PluginAvailableContexts): boolean {
  if (!action.contexts || action.contexts.length === 0) {
    return true;
  }
  return action.contexts.every((context) => availableContexts[context] === true);
}

function expandArg(arg: string, values: PluginActionResolutionValues, missingTokens: string[]): string {
  return arg.replace(tokenPattern, (_match, token: string) => {
    const value = tokenValue(token, values);
    if (value === undefined || value === null || value === "") {
      missingTokens.push(token);
      return "";
    }
    return String(value);
  });
}

function tokenValue(token: string, values: PluginActionResolutionValues): string | number | boolean | null | undefined {
  switch (token) {
    case "projectRoot":
      return values.projectRoot;
    case "workspaceFolder":
      return values.workspaceFolder;
    case "activeFile":
      return values.activeFile;
    case "activeDb":
      return values.activeDb;
    case "outputDir":
      return values.outputDir;
    default:
      if (token.startsWith("input.")) {
        return values.inputs?.[token.slice("input.".length)];
      }
      return undefined;
  }
}
