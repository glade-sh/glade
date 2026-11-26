---
pageType: compatibility
canonicalTask: /guide/known-limitations
---

# Known limitations

Use this list to decide which parts of a project need a Salesforce check.
The [support map](/guide/support-map) describes what runs locally and links to
the checks for each release.

## API versions and hosted services

- Apex source support covers API 62.0–68.0. The LWC and Visualforce preview
  window is 59.0–67.0. HTTP Execute Anonymous accepts source API 65.0–68.0.
  See the [Quickstart version table](/guide/quickstart#api-versions-are-separate-contracts)
  for the HTTP endpoint versions and the LWC compiler's separate version rules.
- Salesforce handles live OAuth, deployment and retrieval, hosted Tooling
  objects, approval routing, Experience Cloud configuration, and email,
  notification, and event delivery. Local event tests exercise the journal
  and subscribers. They cannot verify delivery timing in an org.
- DataWeave and Slack are unsupported. ConnectApi types and selected test
  behaviors are available locally. Its hosted services require Salesforce.

## Apex and test data

- Glade estimates Apex CPU time from work performed. Use Salesforce to check
  exact governor accounting. A bare anonymous `@future` call runs inline.
- Asynchronous SOAP mocks cover string responses. Other response schemas and
  hosted SOAP transport are outside that path.
- Local tests isolate the standard price book for the documented cases. Check
  other pre-existing price books and org records used through `SeeAllData=true`
  in Salesforce.
- Managed-package nested class lookup, reference-only object metadata, and
  commerce contract-resolution types have version- and context-specific coverage.
- Some hash codes, locale rules, and JSON edge cases follow specific captured
  inputs. Verify code that depends on uncaptured record shapes, locale aliases,
  or null JSON map keys in nested values.

## Permissions and sharing

- Without profile metadata, local custom-object and field permissions can be
  more permissive than a Salesforce Standard User's. Use explicit
  [test-user permissions](/reference/config#org-features-and-test-permissions) and validate
  default-deny behavior in Salesforce. Some field-stripping and query errors
  also differ.
- `glade exec` treats deployed permission sets that grant a field as assigned
  to the running administrator. Use `glade test` with `test.permissionSets`
  when a local test needs to control assignment.
- Anonymous record-access queries, permission-set muting, and some user-mode
  query and search errors have limited coverage. Check exact permission behavior
  with the users and profiles you deploy.

## Automation and queries

- Assignment rules cover selected Lead and Case metadata. Local Flow omits
  screen and wait nodes in the Apex runtime, after-delete flows, and hosted
  scheduling. Check before-delete and action paths with currency variables
  in Salesforce. Formula precedence and action outputs also have limited
  coverage. Anonymous Flow errors can omit Salesforce's per-run error ID.
- Duplicate rules cover selected same-object fuzzy methods and a limited
  cross-object mapping. Person Account rules, several mapped fields,
  fuzzy items under OR, and some standard fuzzy rules remain outside local
  coverage. See the [duplicate matching guide](https://github.com/glade-sh/glade/blob/main/docs/DUPLICATE_FUZZY_MATCHING.md)
  for the exact limits.
- Glade follows captured null ordering on specific standard lookup fields.
  An unlisted standard lookup can still honor `NULLS FIRST` or `NULLS LAST`
  where Salesforce ignores it. Use `ORDER BY` when results need a stable
  order. Only specific Account and Contacts query shapes reproduce Salesforce
  index order without it.
- Plain text writes trim spaces and tabs for the covered fields. Check other
  control characters in Salesforce if your application relies on them. Project
  Address fields can read null when their component fields are unlinked.
- Translated SOQL and metadata matching cover specific fields and object
  shapes. Verify translations, package record types, and installed-package
  change events in Salesforce.

## LWC and Visualforce previews

- Use an org to verify Lightning Experience behavior, permissions, base
  components, and full UI API responses. Salesforce uses random IDs in some
  LWC record errors.
- Visualforce previews have local markup, lifecycle, and PDF limits.
  Server-relative links can remain relative locally and include the org
  host in Salesforce. Use an org to verify hosted rendering and exact PDF
  output.

## Verification limits

- The Apex API 62.0 release-note inventory is missing one article. The
  [release notes](https://github.com/glade-sh/glade/blob/main/docs/RELEASE_NOTES.md#v030)
  record the gap.
- Visualforce browser conformance checks require Node.js, Playwright, and
  Chromium. An incomplete CodeQL scan leaves code unscanned. See
  [Security & trust](/guide/security-trust) for the scan limits.

For more detail, see the [catalog gap report](https://github.com/glade-sh/glade/blob/main/docs/KNOWN_GAPS.md)
and the [release notes](https://github.com/glade-sh/glade/blob/main/docs/RELEASE_NOTES.md).
