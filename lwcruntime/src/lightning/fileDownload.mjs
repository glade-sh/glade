export function generateUrl(recordId) {
  return recordId ? `/lightning/r/ContentDocument/${encodeURIComponent(recordId)}/view` : undefined;
}
