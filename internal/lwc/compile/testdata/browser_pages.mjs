// Page count for local browser drivers whose rows are independent. Each page
// is one LWC app boot, so hosts can bound it:
//   GLADE_LWC_BROWSER_PAGES        pages per API pass, set explicitly
//   GLADE_LWC_BROWSER_CONCURRENCY  set on memory-tight hosts; keeps the legacy count
// Without either, the driver uses its wide default.
export function pageCount(legacy, wide, rows, env = process.env) {
  const explicit = env.GLADE_LWC_BROWSER_PAGES;
  let count;
  if (explicit !== undefined && explicit !== "") {
    count = Number(explicit);
    if (!Number.isInteger(count) || count < 1) {
      throw new Error("GLADE_LWC_BROWSER_PAGES must be a positive integer: " + explicit);
    }
  } else if (env.GLADE_LWC_BROWSER_CONCURRENCY !== undefined && env.GLADE_LWC_BROWSER_CONCURRENCY !== "") {
    count = legacy;
  } else {
    count = wide;
  }
  return Math.min(count, rows);
}
