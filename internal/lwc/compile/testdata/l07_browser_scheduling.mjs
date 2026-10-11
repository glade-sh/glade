// Each API keeps its native Apex cache history and alternating LDS pages.
// Only independent pages overlap; the existing three-page limit is unchanged.
export const l07PageCount = 3;

export function createL07PageGroups(fixtures) {
  const pages = fixtures.map(fixture => {
    const groups = [[], [], []];
    let recordIndex = 0;
    for (const spec of fixture.specs) {
      const index = spec.adapter === "apex" ? 0 : 1 + recordIndex++ % 2;
      groups[index].push(spec);
    }
    return groups.map((specs, index) => ({ api: fixture.api, url: fixture.url, index, specs }));
  });
  // Start both long Apex histories first. Free slots drain the shorter LDS
  // histories without splitting a page or moving a row to another cache.
  return [
    ...pages.map(groups => groups[0]),
    ...pages.flatMap(groups => groups.slice(1)),
  ];
}

export async function runL07PageGroups(groups, run) {
  let next = 0;
  await Promise.all(Array.from({ length: l07PageCount }, async () => {
    while (next < groups.length) {
      const group = groups[next++];
      await run(group);
    }
  }));
}
