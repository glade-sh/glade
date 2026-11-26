# Known Gaps

Generated from the first-party compat plugin capability catalog.

These areas are unsupported or partly supported in the local runtime.
See the [known limitations](https://glade.sh/guide/known-limitations) for a guide to current limits.

## Automation

### `automation.assignment.scope`: Assignment rule scope

- Status: `unsupported`
- Gap: The local assignment rule implementation is bounded to the supported Lead and Case metadata and checked selection paths; it does not establish full hosted assignment behavior.

### `automation.duplicate.matching`: Duplicate matching scope

- Status: `unsupported`
- Gap: Local duplicate matching enforces custom same-object rules with Exact items and the First Name, Last Name, Company Name, Phone, City, Street, ZIP and Title fuzzy methods, reproducing native verdicts and match confidences for the captured rows. It enforces a custom cross-object rule whose `objectMapping` maps fields of the rule's object onto the matching rule's object, for the captured one-field mapping (unmapped fields count as blank; mappings of several fields stay unenforced), and Allow+Report rules commit one record set for records that match each other. It does not enforce Person Account custom rules or other cross-object shapes, reports no `getFieldDiffs()` entries for custom rules with fuzzy items or for Person Account rules, models the update gate against insert-time values only for records inserted as an allowed duplicate in the same transaction (a record inserted without a match, moved away and back after another record took its original value, compares with its stored value and is blocked; no native row shows that case, nor do other repeat edits, standard-rule update reports or flow-triggered update reports), leaves `runAsCurrentUser` unmodeled, and leaves standard Account, Contact and Lead fuzzy equations, fuzzy items under OR and rules with three or more fuzzy items unenforced. Metaphone 3 and Syllable Alignment are approximated, so some native matches are missed. See [DUPLICATE_FUZZY_MATCHING.md](DUPLICATE_FUZZY_MATCHING.md).

### `automation.flow.hosted`: Hosted Flow and unsupported nodes

- Status: `unsupported`
- Gap: Screen and wait nodes, after-delete Flow and hosted scheduling are outside the local Flow contract.

## DML

### `dml.text-write-trim.scope`: Text write trimming set

- Status: `unsupported`
- Gap: Plain Text writes trim space and tab, and a value empty afterwards becomes null (captured on Account Name and Site and custom Name). Base already trims spaces on STRING fields. Other control characters (CR, LF, VT, FF, NUL) were not captured and keep base behavior. A blank Name was not captured; Glade keeps REQUIRED_FIELD_MISSING.

## Events

### `events.change.delivery`: Change-event hosted delivery

- Status: `unsupported`
- Gap: Local change-event journal and subscriber execution do not establish hosted event streaming or delivery timing.

## Limits

### `limits.exact-salesforce-accounting`: Exact hosted Salesforce governor accounting

- Status: `unsupported`
- Gap: Exact Salesforce runtime counter deltas require hosted execution accounting and remain outside deterministic local execution.

## Local API server

### `server.rest-breadth.hosted-auth-live-org-deploy`: Hosted auth, live-org Tooling, and deployment services

- Status: `unsupported`
- Gap: OAuth token issuance, live org-only Tooling objects, live metadata deploy/retrieve, and broader hosted REST namespaces require Salesforce services.

## Platform

### `platform.exclusions`: Excluded platforms

- Status: `unsupported`
- Gap: DataWeave and Slack are excluded and unsupported. ConnectApi types are passive
  shapes; its hosted services are not implemented (see release policy).

## SOQL

### `soql.default-order.scope`: Default order without ORDER BY

- Status: `unsupported`
- Gap: Only the captured shapes return index order: an Account query whose entire WHERE clause is a Name prefix LIKE (ascending Name), and an Account to Contacts child subquery whose entire WHERE clause is a LastName prefix LIKE (descending LastName). Compound filters, other child objects, Name on children, GROUP BY, custom objects and describe order keep Id order and are not fitted.

## Security

### `security.default-permissions.scope`: Default permission scope

- Status: `unsupported`
- Gap: Local default permission behavior covers the checked configurations and does not establish full hosted permission parity.
