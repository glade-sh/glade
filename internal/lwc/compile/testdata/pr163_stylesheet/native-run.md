A fresh 30-day Developer scratch org was created for this capture and deleted after use.
Capture date: 2026-11-09 UTC.
Route: Salesforce CLI project deployment of synthetic LightningComponentBundle metadata.
Each deployment used the listed API version in both deployment and bundle metadata.
The probe component uses probe.js, probe.html and probe.css. The css_valid cases
also deploy the CSS-only sharedStyles bundle from sharedStyles.css. Both bundle
metadata files set isExposed false. In css_missing, probe.css instead imports
c/absentStyles, and no such bundle was deployed. Results record deployment
outcomes, not browser rendering.
