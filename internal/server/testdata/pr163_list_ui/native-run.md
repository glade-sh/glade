A fresh 30-day Developer scratch org was created for this capture and deleted after use.
Capture date: 2026-11-09 UTC.
Synthetic list views were deployed to Account and Contact using API 67.0. Each
used the same view metadata pattern as the adjacent list-shapes fixture, with
the listed column substituted. The UI API list-info route was read at 67.0.
The result records the fieldApiName values in displayColumns, not all implicit
fields returned by the separate REST list-view describe route.
