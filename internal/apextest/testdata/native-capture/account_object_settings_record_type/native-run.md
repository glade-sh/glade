Captured 2026-11-07 in a fresh one-day Developer scratch org using the project scratch definition. The same org supplied the account-delete capture. Org deletion was verified.

Account RecordTypeId in a declared Developer shape whose scratch definition sets objectSettings.account.defaultRecordType.
Setup: LpRtProbe and LpRtProbeTest deployed. No record type, profile or permission set was deployed; LpAcct comes from the scratch definition.
Source APIs: 62.0 and 67.0. The classes were redeployed with that apiVersion before each run.
Commands: sf apex run --api-version <api> --file anon/<probe>.apex (rt, s1..s6); sf apex run test --api-version <api> --class-names LpRtProbeTest --synchronous (the test fails on purpose; its message carries the rows).
S1..S6 are compile probes: the "compile" row is true, or false with the compile problem text (whitespace collapsed).
Extraction masks record type Ids as <recordTypeId>.
Findings:
 Account has RecordTypeId: static read, write, SOQL, the field token and the RecordType relationship compile (S1..S5); get/put work, get is null on a new record; describe lists RecordTypeId (REFERENCE to RecordType, createable, updateable, nillable, not defaultedOnCreate).
 getRecordTypeInfos: LpAcct (not available, not default, not master, active) and Master (available, default, master). RecordType has one Account row, LpAcct, active.
 Insert without RecordTypeId succeeds with RecordTypeId null.
 Contact keeps the plain shape: static RecordTypeId does not compile, put raises SObjectException.
 Class API version changes DML: with the probe class at 62.0 through 66.0 (bisected 63.0, 64.0, 65.0, 66.0), an explicit null RecordTypeId raises INVALID_CROSS_REFERENCE_KEY and the unavailable LpAcct inserts; at 67.0 the explicit null inserts and LpAcct raises INVALID_CROSS_REFERENCE_KEY ("Record Type ID: this ID value isn't valid for the user: <id>"). The anonymous --api-version does not change it; the class apiVersion does.
The istest rows equal the anonymous rows at each API.
native_complete=true
