Owned synthetic capture, 2026-11-08 UTC (v0.3.6): Task.WhatId and Event.WhatId targets, a further tiebreak.
Three fresh one-day scratch orgs, each from a different campaign Dev Hub. Release: max API 68.0 in all three.
 P: {"orgName":"gaps16 plain","edition":"Developer"}.
 E: {"orgName":"gaps16 enterprise","edition":"Enterprise"}.
 R: Developer, country US, en_US, feature EnableSetPasswordInApi, lightningExperienceSettings.enableS1DesktopEnabled=true.
Nothing was deployed to P or E. R's rows were taken after the record_type_null_routes objects were deployed to it. Org IDs and instance names are not retained.
Routes, each at API 62.0 and 67.0:
 anonymous:    sf apex run --api-version <api> --file whatid_targets_tiebreak/project/anon.apex (sorted lists).
 restDescribe: sf api request rest /services/data/v<api>/sobjects/<Task|Event>/describe, WhatId referenceTo (as returned).
Findings:
 P and R: 85 targets at 62.0 and 93 at 67.0, on both routes, Task and Event alike. The eight objects that differ
 are absent from the 62.0 global describe. This agrees with whatid_targets_tiebreak orgs A and B and disagrees with
 the plain_global_describe 62.0 isTest row (93), which stays as captured and stays the minority.
 E: 27 at 62.0 and 33 at 67.0, a subset of the Developer lists; six of the eight objects at 67.0. Edition changes the list.
Cleanup: all three orgs deleted (sf org delete scratch); ScratchOrgInfo Status recorded as Deleted.
