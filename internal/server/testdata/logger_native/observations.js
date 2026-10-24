export function observeLogger(log) {
  const cyclic = {}; cyclic.self = cyclic;
  const cases = [
    ['string', 'synthetic telemetry'], ['empty', ''], ['object', { action: 'save', count: 2 }],
    ['null', null], ['undefined', undefined], ['number', 7], ['boolean', true],
    ['array', ['entry', 2]], ['cyclic', cyclic], ['function', () => 'entry'],
    ['bigint', BigInt(1)]
  ];
  const rows = [['export', typeof log]];
  for (const [id, value] of cases) {
    try { rows.push([id, typeof log(value)]); }
    catch (error) { rows.push([id, error.name + ': ' + error.message]); }
  }
  try { rows.push(['no_arguments', typeof log()]); }
  catch (error) { rows.push(['no_arguments', error.name + ': ' + error.message]); }
  return rows;
}
