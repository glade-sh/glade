// Keep L19's existing page limit and ordinary native batch order. Controls
// own fresh pages and records per row, including any same-page remount primer.
export const l19PageCount = 6;

export function createL19BatchAllocator(rows, controlProject) {
  const ordinaryOffsets = Array.from({ length: l19PageCount }, (_, worker) => worker * 15);
  let nextControl = 0;
  return worker => {
    if (!Number.isInteger(worker) || worker < 0 || worker >= l19PageCount) {
      throw new RangeError("L19 page worker is outside the bounded pool");
    }
    const index = controlProject ? nextControl++ : ordinaryOffsets[worker];
    if (index >= rows.length) return null;
    if (!controlProject) ordinaryOffsets[worker] += l19PageCount * 15;
    return { index, rows: rows.slice(index, index + (controlProject ? 1 : 15)) };
  };
}
