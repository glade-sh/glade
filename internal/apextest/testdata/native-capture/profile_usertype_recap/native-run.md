Captured 2026-11-10 in a fresh one-day Developer Edition scratch org without a namespace. Org deletion was verified.

Route: REST SOQL (sf data query --api-version 62.0 and 67.0), not Apex. Probe P1: SELECT Name, UserType, UserLicense.Name, UserLicense.LicenseDefinitionKey FROM Profile ORDER BY Name. Probe P2: SELECT Profile.UserType FROM PermissionSet WHERE IsOwnedByProfile = true.

org.tsv columns: source_api, probe, name, user_type, license_name, license_key (P2 rows carry only user_type).

Findings (identical at 62.0 and 67.0): 38 profiles and 38 profile-owned permission sets. Chatter External User is CsnOnly on license Chatter External (CSN_External_User); Standard Platform User is Standard on Salesforce Platform (AUL); there is no Customer Community Guest User profile. The P1 profile/UserType/license set is identical to the earlier conformance/profile_usertype/org.tsv rows (Apex anonymous and @isTest routes at the same two APIs), which carried no native-run.md; this recapture supplies their provenance for the profile and license facts.
Not captured: Apex routes (the earlier rows cover them), API 63.0-66.0, namespaced orgs, access rules of any license.
