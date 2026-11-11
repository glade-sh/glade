# flow_record_merge native run

Custom Error merge fields of `$Record` Number, Currency, Percent, DateTime and Time
fields, and of flow variables, constants and formulas, at API 62.0 and 67.0.

Routes: anonymous Apex (`sf apex run --api-version`, one file per case; the observation
is the DML message from a `JSON.serialize` debug line, with the debug log's `&#124;`
read back as `|`) and @IsTest (`MfProbe`, `sf apex run test`, the observation is the
assertion message). Both routes are in `org.tsv`; they agree on every row.

Org: 1-day Developer scratch org, deployed `project/`, permission set assigned to the
running administrator, deleted afterwards. A "Mf Err NN" flow (before-save, create or
update, filter Name = ERRNN) throws a Custom Error whose message merges seven `$Record`
fields. The `Mf Var NN` flow merges Number variables, constants and a formula.
A fault row ends with the platform's `Error ID: n (n)`; that suffix is removed.
