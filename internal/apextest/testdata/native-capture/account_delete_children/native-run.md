Captured 2026-11-07 in a fresh one-day Developer scratch org using the project scratch definition. The same org supplied the account object-settings capture. Org deletion was verified.

Each scenario runs inside a savepoint that is rolled back. Child state is read using SELECT with ALL ROWS: live, deleted, or gone. The probe masks record IDs, ContractNumber, and CaseNumber; extraction masks case numbers the same way and trims the trailing newline from delete-failure messages.

Account delete with Contracts, Opportunities, Cases, Contacts and Orders attached.
Setup: project/force-app deployed (LpDeleteProbe, LpDeleteProbeTest). Nothing else was deployed or configured.
Source APIs: 62.0 and 67.0. The classes were redeployed with that apiVersion before each run.
Commands: sf apex run --api-version <api> --file anonymous.apex; sf apex run test --api-version <api> --class-names LpDeleteProbeTest --synchronous (the test fails on purpose; its message carries the rows).
Findings:
 Contract: Draft and In Approval Process contracts are deleted with the Account (cascade) and restored by undelete. An Activated contract (StatusCode Activated, also with an end date in the past) blocks it: DELETE_FAILED, no fields, "Your attempt to delete <Account name> could not be completed because it is associated with the following active contracts.: <ContractNumber>". Status 'Expired' is not settable (INVALID_OR_NULL_FOR_RESTRICTED_PICKLIST). A contract itself deletes in either status.
 Opportunity: an open opportunity is deleted with the Account. A Closed Won one blocks it: "...because some opportunities in that account were closed won. The opportunities that could not be deleted are shown below.: <Opportunity name>".
 Case: any case on the Account (open or closed, or one that took its AccountId from its Contact) blocks it: "...because it is associated with the following cases.: <CaseNumber>".
 Contact and Draft Order: deleted with the Account; a Draft Order on a Draft Contract goes with both.
 A blocked delete leaves every child live (all or none); Database.delete(record, false) returns the same error.
The two APIs agree on every row, and the istest rows equal the anonymous rows.
native_complete=true
