Capture date 2026-11-10. Seven fresh 30-day Developer scratch orgs on campaign Dev Hubs, one org per arm, each deleted after capture (the delete command returned success for every org).

Question: does Salesforce select the same overload when only declaration order, deploy order, class-name layout, Apex API version, caller component API version or the repeat changes?

Synthetic project: eight classes, each declaring equal-arity `pick(String value)` and `pick(Integer value)` AuraEnabled overloads. Apex API 45 and 54 (the highest API that still deploys overloaded AuraEnabled methods); declaration order String-first and Integer-first; two class-name positions (A and Z). A control class has one method. Three Lightning web components at API 59, 62 and 67 imported every method and called it through a Visualforce-hosted Lightning Out application with missing, null, numeric, string, boolean and decimal parameters, each three times. Each component saved its results into synthetic records that were read through the API.

Arms: a1, a5 and a4 bulk deploy of every class together (a4 also attempted a redeploy that rolled back, so it is a third bulk repeat); a2 sequential per-class deploys in forward order; a3 and a6 sequential per-class deploys in reverse order; a7 bulk deploy followed by a redeploy of every class. Orgs a1..a7 were created on different Dev Hubs.

Result: all 3,402 observations selected the String overload (and converted numeric, boolean and decimal values to strings). No cell varied. The metadata-deploy rows record that overloaded AuraEnabled declarations are rejected at Apex API 62.0 and 67.0, as the earlier API 55 and 56 observations did.
