import { LocalOrgSummary } from "./localOrgModel";
import { StartHereRunSummary } from "./startHereModel";

export interface StartHereRuntimeSnapshot {
  watchRunning: boolean;
  lastRun?: StartHereRunSummary;
  localOrgSummary?: LocalOrgSummary;
  missingDb?: boolean;
  toolchainReady?: boolean;
  toolchainDetail?: string;
  lwcRouteCount?: number;
  vfRouteCount?: number;
  pluginActionCount?: number;
}

export class StartHereState {
  private watch = false;
  private run?: StartHereRunSummary;
  private summary?: LocalOrgSummary;
  private dbMissing?: boolean;
  private toolchain?: boolean;
  private toolchainMessage?: string;
  private lwcRoutes?: number;
  private vfRoutes?: number;
  private pluginActions?: number;

  setWatchRunning(running: boolean): void {
    this.watch = running;
  }

  setLastRun(run: StartHereRunSummary): void {
    this.run = run;
  }

  setLocalOrgSummary(summary: LocalOrgSummary | undefined): void {
    this.summary = summary;
  }

  setMissingDb(missing: boolean | undefined): void {
    this.dbMissing = missing;
  }

  setToolchainStatus(ready: boolean | undefined, detail?: string): void {
    this.toolchain = ready;
    this.toolchainMessage = detail;
  }

  setPreviewCounts(counts: { lwcRouteCount?: number; vfRouteCount?: number }): void {
    this.lwcRoutes = counts.lwcRouteCount;
    this.vfRoutes = counts.vfRouteCount;
  }

  setPluginActionCount(count: number | undefined): void {
    this.pluginActions = count;
  }

  snapshot(): StartHereRuntimeSnapshot {
    return {
      watchRunning: this.watch,
      lastRun: this.run,
      localOrgSummary: this.summary,
      missingDb: this.dbMissing,
      toolchainReady: this.toolchain,
      toolchainDetail: this.toolchainMessage,
      lwcRouteCount: this.lwcRoutes,
      vfRouteCount: this.vfRoutes,
      pluginActionCount: this.pluginActions,
    };
  }
}
