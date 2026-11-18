# Native route inventory

The rows were captured on 2026-11-14 and 2026-11-15 UTC in fresh Developer
scratch orgs with 30-day durations and no features or settings. Each scratch org
was deleted and its absence was verified both by org lookup and by the Dev Hub.
The captured runs used Salesforce CLI 2.157.8. The persistent SOAP and REST
clients were checked against the CLI using positive and negative controls.

The first anonymous batch was captured at 22:15 UTC; a resumed run captured
344 case/API inputs from 22:19 to 22:48 UTC. The remaining anonymous, test,
Visualforce, UI batch, REST and some singleton observations were captured from
23:25 to 23:50 UTC. Singleton UI observations resumed in a fresh scratch org
from 23:55 to 00:00 UTC; previously completed singleton checkpoints were
retained. Validated singleton checkpoints were not repeated; controls and incomplete batches were retried. The separate
virtual metadata and API 68 global-describe probes ran in the 23:25 run.

`org.tsv` records API, route, object and up to four outcome fields. For Apex and
Visualforce these are `Type.forName` and dynamic query outcomes; for REST they
are describe status/error and query status/error. UI batch rows contain status
and error; singleton rows contain presence, error and the returned canonical
API name when present; `-` marks a field that does not apply. `entity_definition` and `global_describe` rows contain a
presence boolean. The previously captured anonymous API 62 and 67 rows for
three objects remain in `../plain_developer_spread/org.tsv` and are not repeated.
`virtual.tsv` records the additional virtual-schema query outcomes; `access.tsv`
records the follow-up checks for the two access tables. Query exceptions are
outcomes, not proof that an object or access record is absent.

The `cases-*.tsv` files identify captured Apex, Visualforce and metadata inputs.
The `source` directory contains the synthetic inputs: `anonymous-before` for
earlier frames, `anonymous-caught` for later frames, `visualforce` for page
wrappers, `force-app` for test classes and page controllers, and `entity` and
`global` for metadata probes. `virtual.apex` and `access.apex` are the additional
virtual-schema probes. Every batch includes the Account positive control and a
synthetic missing-object negative control.

For REST and UI inputs, `objects.txt` is the name manifest. Remove the three
objects already covered by the earlier fixture, retain the manifest order and
append those three names in sorted order. REST composite requests group five
names, with one describe and one `SELECT Id ... LIMIT 1` query per name, at API
60, 65, 66, 67 and 68. UI batch requests group twenty names after the two
controls at API 59, 67 and 68. Singleton UI requests use each manifest name at
the same three APIs, with the controls repeated between groups of twenty.

A separate fresh 30-day Developer scratch org captured the missing Visualforce API 60–66 boundaries using the controllers, pages and wrappers under `source/vf-gap`. Glade preflight preceded the capture. Positive and negative controls passed. The scratch org was deleted and its absence verified through the Dev Hub. These rows append to `org.tsv`; earlier observations remain unchanged.
