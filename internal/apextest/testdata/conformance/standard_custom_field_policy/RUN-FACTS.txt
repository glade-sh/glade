Standard User runAs DML on a custom field of a standard object (Account) with and without field-level security.
org: fresh scratch org from a campaign Dev Hub, Developer Edition, deleted after the run (delete verified). Org id, usernames, alias and host are not recorded.
capture date: 2026-11-13 (UTC). sf 2.157.8. Class source API 62.0 and 67.0 (class apiVersion), deployed at 67.0.
metadata: Account.Probe_A__c and Account.Probe_B__c, Text(255); permission set Probe_Access grants edit on Probe_A__c only. User A has the permission set, user Anops does not, user B writes Probe_B__c, which no one is granted.
route: named @IsTest methods, one transaction each. Two fresh orgs, each run twice: all four runs agree on every row (36 methods x 2 APIs). The org.tsv holds one row per method and API.
rows: insert and update by statement, default and explicit USER_MODE or SYSTEM_MODE, upsert, Database.insert allOrNone true and false with default, USER_MODE and SYSTEM_MODE, and a two-row list in which one row carries the field.
observation: text after "=>" in the assertion message; "ok" means no exception; "fail <status>:<message>" is a failed SaveResult.
