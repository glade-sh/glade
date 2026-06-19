# Glade Home prototype

This folder holds a standalone UI prototype for the Glade Home VS Code hub.
Open `local-org-dashboard.html` in a browser.

The page sketches one operational view:

- project root, active Glade org, active data environment, schema, and changed-test
  state at the top;
- a setup rail for project, local org, Salesforce target, data, and changed tests;
- one command strip for inspect, seed, reset, export, SOQL, anonymous Apex, and
  changed tests;
- a connection and data movement area for Salesforce target checks, schema import,
  fixture capture, and fixture seed selection;
- a database browser with object filtering, row preview, SOQL scratch, and
  result preview.

Current extension commands can back several buttons now: `glade.inspectLocalOrg`,
`glade.seedLocalOrg`, `glade.resetLocalOrg`, `glade.exportLocalOrg`,
`glade.runLocalProof`, `glade.workbench.newSoql`,
`glade.workbench.newAnonymousApex`, `glade.runSoql`,
`glade.workbench.openResult`, and `glade.workbench.describe`.

Future work should keep live Salesforce capture as a plugin action. Core Glade
can own the local org, schema import, fixture seed, local DB browser, SOQL
scratch, and changed-test evidence.

The real extension should treat Home as the default task surface and State as
the read-only board for project, org, data, test, Salesforce, and plugin state.
