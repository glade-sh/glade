A fresh 30-day Developer scratch org was created for this capture and deleted after use.
Capture date: 2026-11-09 UTC.
Route: Salesforce CLI project deployment of a synthetic LightningComponentBundle.
Each deployment used the listed API version in both deployment and bundle metadata.
The component source is probe.js and probe.html. Bundle metadata set isExposed false;
the dynamic_cap cases additionally declared lightning__dynamicComponent. The
otherwise identical dynamic_no_cap cases omitted the capabilities element.
A rejected deployment reported LWC1503. Results record deployment outcomes,
not browser execution.
