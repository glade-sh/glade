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
