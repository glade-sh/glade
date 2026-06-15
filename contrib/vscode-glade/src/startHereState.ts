import { LocalOrgSummary } from "./localOrgModel";
import { StartHereRunSummary } from "./startHereModel";

export interface StartHereRuntimeSnapshot {
  watchRunning: boolean;
  lastRun?: StartHereRunSummary;
  localOrgSummary?: LocalOrgSummary;
  missingDb?: boolean;
  pluginActionCount?: number;
}

export class StartHereState {
  private watch = false;
  private run?: StartHereRunSummary;
  private summary?: LocalOrgSummary;
  private dbMissing?: boolean;
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

  setPluginActionCount(count: number | undefined): void {
    this.pluginActions = count;
  }

  snapshot(): StartHereRuntimeSnapshot {
    return {
      watchRunning: this.watch,
      lastRun: this.run,
      localOrgSummary: this.summary,
      missingDb: this.dbMissing,
      pluginActionCount: this.pluginActions,
    };
  }
}
