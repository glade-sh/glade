export function formatDateTimeUTC(value) {
  return new Date(value).toISOString();
}
export function formatDateUTC(value) {
  return new Date(value).toISOString().slice(0, 10);
}
export function parseDateTimeUTC(value) {
  return new Date(value);
}
export function syncUTCToWallTime(value) {
  return new Date(value);
}
export function syncWallTimeToUTC(value) {
  return new Date(value);
}
export function addressFormat(parts = {}) {
  return [parts.street, parts.city, parts.province, parts.postalCode, parts.country].filter(Boolean).join(", ");
}
export function nameFormat(parts = {}) {
  return [parts.salutation, parts.firstName, parts.middleName, parts.lastName, parts.suffix, parts.informalName].filter(Boolean).join(" ");
}
