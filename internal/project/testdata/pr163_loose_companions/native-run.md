A fresh 30-day Developer scratch org was created for this capture and deleted after use.
Capture date: 2026-11-09 UTC.
Route: Salesforce CLI source deployment of a synthetic lwc directory.
Each loose case placed the corresponding invalid source directly under lwc and
deployed a valid probe bundle. Each bundle_invalid case retained that loose file
and placed the same invalid source at the bundle matching filename. The bundle
contains probe.js, probe.html, probe.css, and probe.js-meta.xml; metadata uses
the row API and isExposed false. The deployment command also uses the row API.
This records CLI source selection and deployment, not browser execution.
