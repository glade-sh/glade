# Empty reference SUM locator probes

Twelve independently authored inputs cover construction and iteration separately
for empty, deleted, filtered and matching reference-ID bindings, plus grouped
COUNT, a matching grouped COUNT control, ungrouped SUM and grouped cursor inputs.
They use only standard Account and Opportunity data created by the input itself.

`probe-cases.json` embeds every anonymous body and named observer class. The
`.apex` files and `project/` are runnable copies. Use source API 62.0, 67.0 and
68.0 on anonymous and named test routes: 72 cells. Change both project and class
metadata before each named capture. Named terminal assertions intentionally
transport the payload; a test failure carrying HANDLE_OBSERVATION is not a
predicted behavioral failure. None of these new cells has a native expectation.

Payload `phase` distinguishes setup, construction, iteration and completion.
Retain exact compiler rejection and runtime exception type/message. Do not
replace failures with inferred answers. The matching COUNT control refreshes
owned query_locators.json N015; its API68 routes need fresh native observation.
The local construction test is a pending compatibility candidate, not parity.
The implementation deliberately rejects its iterator until iteration is captured.

The coordinator owns serial validation. `run-before-after.sh BEFORE AFTER OUTPUT`
archives both refs into new temporary directories, brings the same synthetic
regression file to BEFORE, expects the construction regression to fail there,
and runs the three focused tests on AFTER. It runs no build, corpus or broad lane.
No worker has run this script or the probes.
