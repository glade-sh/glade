# Native capture: loop-body locals and dynamic SOQL bind scope

Captured 2026-11-11 in a fresh Developer Edition scratch org (deleted after the
run) with Salesforce CLI 2.156.8, at API 62.0 and 67.0.

Routes:

- anonymous: `sf apex run --api-version <api>`; `anon/<id>.apex`.
- named_class: one `@IsTest` class per case (`classes/<Class>.cls`, class
  metadata at the row's API version) deployed with `--ignore-errors`, then run
  with `sf apex run test`. A class the org rejects at deploy is a
  `compiled=false` row with the deploy problem text. A class that compiles ends
  its test with `System.assert(false, outBuf)` so the buffer shows in the
  failure message; an uncaught exception shows its own message instead.

`org.tsv` columns: `id`, `api`, `route`, `compiled`, `observation`.
`observation` is the `OUT:` debug lines joined by `|` (anonymous), the
`outBuf` text with `;OUT:` written as `|OUT:` (named_class), the compile
problem text, or `EXC:<type>: <message>` for an uncaught exception.

Cases:

- `l*`, `f*`: locals declared in `while`, `do`, `for` and `if` bodies, read or
  redeclared after the body, with and without a class field of the same name.
- `d*`, `n*`, `p*`: `Database.query` with a dynamic string whose `:n` binds a
  String local that is declared before the query, declared later in the same
  block, declared in a block that has ended, declared in an enclosing block,
  shadowed, re-entered by a loop, or hidden by a class field or parameter.

Every 62.0 row equals its 67.0 row. The Account rows `bound`, `fld` and `loc`
are inserted by each script (the case source) before the query.

Observed rules this fixture pins:

- A local declared in a `while` or `do` body ends with the body, as in `for`
  and `if` bodies. Reading it afterwards is `Variable does not exist`; a field
  or an outer local of the same name is visible again.
- Named classes reject a nested-block redeclaration of an enclosing local
  (`Duplicate variable`). Anonymous Apex accepts it, and the outer local keeps
  its value.
- A dynamic bind still sees the last value of a String local whose block has
  ended (normal end, `break`, `continue` or a thrown exception). Entering the
  block again hides it until the declaration runs. A same-named local of an
  enclosing method level does not outrank the ended block's local. A foreach
  variable is visible afterwards as null.
- Anonymous Apex only: a String declared later at the top level binds as null
  before its declaration runs. Named classes throw
  `Variable does not exist`.

Parked (`parked.tsv`): dotted dynamic binds (`:acc.Name`) throw natively even
when `acc` is declared; Glade accepts them.
