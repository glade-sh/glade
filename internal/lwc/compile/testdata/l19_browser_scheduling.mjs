import { pageCount } from "./browser_pages.mjs";

// Ordinary rows share page-local LDS history within each fifteen-row batch.
// Keep their six workers and fixed offsets even though each batch reopens its page.
export const l19PageCount = 6;

// Controls mount c__case in fresh browser.newPage() contexts: cookies/storage/SW,
// component state and module LDS caches cannot carry between rows. REST writes
// and cleanup use fresh UUID-owned records; controls do not search other records.
// Their same-page-remount primer and capture stay together within one row.
export function l19WorkerCount(rows, controlProject, env = process.env) {
  return controlProject ? pageCount(l19PageCount, 12, rows.length, env) : l19PageCount;
}

export function createL19BatchAllocator(rows, controlProject, workers = l19WorkerCount(rows, controlProject)) {
  const ordinaryOffsets = Array.from({ length: l19PageCount }, (_, worker) => worker * 15);
  let nextControl = 0;
  return worker => {
    if (!Number.isInteger(worker) || worker < 0 || worker >= workers) {
      throw new RangeError("L19 page worker is outside the bounded pool");
    }
    const index = controlProject ? nextControl++ : ordinaryOffsets[worker];
    if (index >= rows.length) return null;
    if (!controlProject) ordinaryOffsets[worker] += l19PageCount * 15;
    return { index, rows: rows.slice(index, index + (controlProject ? 1 : 15)) };
  };
}
