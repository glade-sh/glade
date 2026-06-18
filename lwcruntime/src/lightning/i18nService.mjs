function asDate(value) {
  const date = value instanceof Date ? value : new Date(value);
  return Number.isNaN(date.getTime()) ? new Date("") : date;
}

export function clearCache() {}
export function getDateTimeCLDRParser() {
  return { parse: (value) => asDate(value) };
}
export function getDateTimeFormat(options = {}) {
  return new Intl.DateTimeFormat(undefined, options);
}
export function getDateTimeISO8601Parser() {
  return { parse: (value) => asDate(value) };
}
export function getNumberFormat(options = {}) {
  return new Intl.NumberFormat(undefined, options);
}
export function getNumberParser() {
  return { parse: (value) => Number(String(value).replace(/,/g, "")) };
}
export function getRelativeTimeFormat(options = {}) {
  return new Intl.RelativeTimeFormat(undefined, options);
}
