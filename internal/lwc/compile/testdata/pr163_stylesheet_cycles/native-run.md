A fresh 30-day Developer scratch org was used and deleted; a subsequent org
lookup returned NamedOrgNotFoundError. Capture date: 2026-11-09 UTC.
Deployment route: Salesforce CLI deployment of two synthetic CSS-only LWC
bundles and a probe component. The acyclic case imports beta from alpha;
cycle additionally imports alpha from beta; missing imports an absent module.
Browser route: exposed probe tabs imported corresponding acyclic or cyclic
stylesheets and displayed a text marker. Both rendered the listed computed color
without a page error at each listed component API version. The deployment API
and component metadata used the listed version.
