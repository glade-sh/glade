A fresh 30-day Developer scratch org was created for this capture and deleted after use.
Capture date: 2026-11-09 UTC.
Route: Salesforce CLI source deployment of the lwc directory, using the listed API version.
The lwc directory contained invalid.js as a loose JavaScript file and a probe
bundle with probe.html and metadata setting the listed API version and isExposed
false. The loose case used probe.js inside the bundle; bundle_invalid placed the
same invalid source inside the bundle instead. The CLI accepted the loose case
and the invalid bundle case was rejected with an LWC1503 parsing error. The
result describes source deployment, not direct Metadata API treatment of a
loose file.
