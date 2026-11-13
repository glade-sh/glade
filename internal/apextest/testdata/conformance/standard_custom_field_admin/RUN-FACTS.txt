Default test-runner administrator DML and dynamic SOQL on custom fields of a standard object (Account) deployed without a field grant for that user.
org: two fresh scratch orgs from a campaign Dev Hub, Developer Edition, each deleted after the run (delete verified). Org ids, usernames, aliases and hosts are not recorded.
capture date: 2026-11-13 (UTC). sf 2.157.8. Class source API 62.0 and 67.0 (class apiVersion), deployed at 67.0.
metadata: Account.Probe_A__c and Account.Probe_B__c, Text(255), deployed with no profile metadata; permission set Probe_Access grants edit on Probe_A__c only and is not assigned before the run.
route: named @IsTest methods, one transaction each, no System.runAs. Each org ran each class twice; all four runs agree on every row. The org.tsv holds one row per method and API.
methods: a* run as the default administrator; p* first insert a PermissionSetAssignment of Probe_Access for the running administrator. A and B name the field written. ins, upd: insert and update statements; dbit, dbif: Database.insert allOrNone true and false; dbu: Database.update allOrNone false; sel: dynamic SELECT of the field.
observation: text after "=>" in the assertion message; "ok" means no exception; "fail <status>:<message>" is a failed SaveResult; "stored=" is a dynamic SELECT of the field after the operation (readfail: the query threw).
