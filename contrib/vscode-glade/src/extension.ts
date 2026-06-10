import * as vscode from "vscode";
import { adapterExecutable, GladeDebugConfiguration, resolveGladeConfiguration } from "./adapter";
import { registerGladeCommands } from "./commands";

class GladeDebugAdapterFactory implements vscode.DebugAdapterDescriptorFactory {
  createDebugAdapterDescriptor(session: vscode.DebugSession): vscode.ProviderResult<vscode.DebugAdapterDescriptor> {
    return adapterExecutable(session.configuration as GladeDebugConfiguration);
  }
}

export function activate(context: vscode.ExtensionContext): void {
  registerGladeCommands(context);
  context.subscriptions.push(
    vscode.debug.registerDebugConfigurationProvider("glade", {
      resolveDebugConfiguration(folder, config) {
        return resolveGladeConfiguration(folder, config as GladeDebugConfiguration);
      },
    }),
  );
  context.subscriptions.push(
    vscode.debug.registerDebugAdapterDescriptorFactory("glade", new GladeDebugAdapterFactory()),
  );
}

export function deactivate(): void {}
