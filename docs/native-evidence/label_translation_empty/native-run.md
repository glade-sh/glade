Salesforce Developer scratch org; capture date 2026-11-09.
Enabled Translation Workbench, deployed the labels, French translation metadata,
and API 62.0 and 67.0 test classes. Anonymous probes were run at both API
versions. Both methods passed in each class. The rejected case catches the
reported exception in anonymous Apex. A metadata retrieve of the French
translation returned an empty label element for both the explicitly empty and
omitted translations, and the nonempty value for the translated label.
