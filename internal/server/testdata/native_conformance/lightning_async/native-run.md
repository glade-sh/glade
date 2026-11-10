Long-lived personal Developer Edition org (not a fresh scratch org), capture
date 2026-11-10. classes/GladeQueueProbe.cls (API 62.0) was deployed, and each
.apex source ran through executeAnonymous at the API in org.tsv, with <API>
replaced by the major version. After each run the AsyncApexJob was polled until
it finished, and Accounts named glade-queue-probe-<tag> were counted.
inTransactionCount is the count asserted inside the enqueuing transaction;
newJobs counts GladeQueueProbe jobs created by the run. The probe class and its
Accounts were deleted afterwards. No org, user, job or record identifiers are
recorded. The System.debug of the job Id used for polling is omitted from
the .apex sources.
