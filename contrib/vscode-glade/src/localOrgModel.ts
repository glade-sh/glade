export interface DBInspectResult {
  path?: string;
  schemaVersion?: number;
  objects: number;
  records: number;
  byObject?: Record<string, number>;
  users?: number;
  profiles?: number;
  permissions?: number;
}

export interface LocalOrgObjectRow {
  name: string;
  rows: number;
}

export interface LocalOrgSummary {
  objects: number;
  records: number;
  users: number;
  profiles: number;
  permissions: number;
}

export function objectRowsFromInspect(result: DBInspectResult): LocalOrgObjectRow[] {
  return Object.entries(result.byObject || {})
    .map(([name, rows]) => ({ name, rows }))
    .sort((a, b) => a.name.localeCompare(b.name));
}

export function summaryFromInspect(result: DBInspectResult): LocalOrgSummary {
  return {
    objects: result.objects || 0,
    records: result.records || 0,
    users: result.users || 0,
    profiles: result.profiles || 0,
    permissions: result.permissions || 0,
  };
}
