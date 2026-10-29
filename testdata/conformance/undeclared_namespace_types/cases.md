# Undeclared namespace types

Synthetic fixture. The project declares `"namespace": ""` and references types
of namespaces it does not declare: `bar` (an installed managed package) and
`fooNs` (an unknown qualifier, with `fooNs.LocalThing` naming a project class
and `fooNs.Unknown` naming nothing in the project).

Offline Glade cannot see which packages an org has installed or which
namespace the deploying org owns. A miss in its catalogs is not proof of
absence, so these references are deferred. `cases.tsv` lists each case and
the positions it covers.

Baseline: Glade a recorded revision accepted every `accepted` case through check,
anonymous analysis, the test runner and LSP at API 62.0 and 67.0.
`base_boundary/` records the local-declaration and static-call shapes that
a recorded revision already rejected; `base.tsv` records that historical boundary. The
local declarations reject, as in v0.3.1 (a recorded revision) and native A04
C055/N010/N011 in `internal/apextest/testdata/conformance/classes_lifecycle.json`.
The static call `bar.Logger.log()` keeps the
release rejection: offline execution has no method to run, so the calling
unit stays a compile exclusion and independent tests still run.

No Salesforce capture backs these rows. Native rejection of an absent
namespace in an org without the package is recorded separately in
`internal/sema/testdata/qualified_receiver_shapes/org.tsv`.
