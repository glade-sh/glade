Fresh Developer edition scratch org (30 days), 2026-11-11, API 62.0 and 67.0.
The org held a Widget__c custom object (Real__c, Txt__c, Hidden__c, a Parent__c
self lookup), an Account custom field (AcctCustom__c) and a permission set
(ProbeAccess) that grants every custom field except Hidden__c. Metadata is in
project/.

Anonymous route: each case body ran alone through Tooling executeAnonymous at
the row's API version. Named route: each case body became the body of
`public static void run()` in its own class K_<case_id>, deployed together by
metadata deploy at the row's apiVersion. A class the deploy reported as failed
has the deploy's problem text; the accepted classes were deployed again (all
accepted) and each `K_<case_id>.run()` was run through executeAnonymous.

Rows are in ../../../apextest/testdata/conformance/unknown_sobject_field/. Routes: anonymous, named,
trigger (a before-insert trigger on Account or Widget__c; rows T_*) and istest (an
@IsTest class; rows I_*). Orgs in org.tsv:
- plain: the org above.
- plain-granted: the same org after a permission set granted Case.EntitlementId,
  BusinessHoursId, MilestoneStatus (read), IsStopped (read), SlaStartDate (read),
  Contact.GenderIdentity, Contact.Pronouns and Widget__c.Hidden__c to the
  running user. Rows H_*, F_* and K_cont_* only.
- namespaced: a namespaced Developer edition org (its namespace is written nsone in the rows and the project) that has
  own-namespace custom fields on Account, Contact and Case and one installed
  package from a different namespace (written pkgapp). Rows NS_*. Its project is
  project_namespaced/ (field files only). The K_NS_* classes were removed from
  the org afterwards.

Both API versions gave identical answers for every case except H_EntitlementId_soql
on the named route (API 67.0 fails at run time, 62.0 does not).

The scratch org was deleted after capture.

## Case, User and WebStore field surface (added 2026-11-11)

Rows with orgs `case-user-surface` and `webstore-surface` (named route only,
API 62.0 and 67.0). Each row is one uncalled class method compiled by one
metadata deploy per API version; a class in the deploy's failure list carries
that problem text and position, and a class missing from it compiled.

- case-user-surface: a fresh Developer edition scratch org (30 days) with
  Communities, ServiceCloud, MultiCurrency, PersonAccounts,
  StateAndCountryPicklist, ContactsToMultipleAccounts, Knowledge, LiveAgent,
  SocialCustomerService and the entitlement setting, plus a permission set that
  grants read access to every Case field the org exposes (entitlement, SLA,
  milestone, asset warranty and service contract fields).
  Fields read: every Case field of the REST describe, the generated catalog and
  the captured profile deltas, and every User field of the same sources, each
  as `x.Field`, `x.field` (Case, plus a sample of User) and `x.FIELD`.
  Compiled at both APIs: 48 Case names and 198 User names (REST describe at the
  maximal grant: Case 48 fields at 62.0 and 67.0, User 191 at 62.0 and 198 at
  67.0). Rejected in that org: Case.HasCommentsUnreadByOwner,
  HasSelfServiceComments, IsSelfServiceClosed, IsVisibleInSelfService, Language
  and RecordTypeId (no record types), and User.IsPortalSelfRegistered,
  UserPermissionsAvantgoUser, UserPreferencesHideBrowseProductRedirectConfirmation,
  UserPreferencesHideOnlineSalesAppTabVisibilityRequirementsModal and
  UserPreferencesHideOnlineSalesAppWelcomeMat; at 62.0 only, User.HasUserVerifiedEmail,
  HasUserVerifiedPhone and five UserPreferences names that 67.0 adds. These names
  exist in the captured portal and API-version configurations
  (standard_describe_profiles P4, P11) and stay accepted; no row asserts them.
  Case.SourceId compiled in this org and is rejected in the plain org rows
  (K_case_SourceId_read), so it is feature-gated and no S_case_SourceId row is asserted.
  The org did not expose ConnectionReceivedId or ConnectionSentId (partner
  networks); both are documented Case fields and stay accepted unrecorded.
- webstore-surface: a fresh Developer edition scratch org with the
  OrderManagement feature set (standard_describe_profiles PU shape). The REST
  describe has 45 WebStore fields at both APIs. Every sidecar surface name that
  the org exposes compiled; Country, DefaultTaxPolicy(Id) and GuestBuyerProfile(Id)
  did not (other profiles). CurrencyIsoCode and SupportedCurrencies (MultiCurrency)
  compiled although the sidecar surface lacked SupportedCurrencies.
  A trial Commerce org cannot compile Apex ("Apex compilation not enabled"), so
  it gave no rows.
- D_* rows count the reports of one source statement: a compound assignment
  (`+=`, `*=`) on an unknown field is reported twice at the same position, an
  assignment, ++, --, a prefix ++, a read and each distinct constructor field
  once. Glade matches these counts.

Both API versions gave identical answers for every row. Both orgs were deleted
after capture.
