A fresh 30-day Developer scratch org was used for this capture and deleted.
Capture date: 2026-11-09 UTC.
Route: Salesforce CLI deployment of synthetic LightningComponentBundle metadata.
Each deployment included a child component and a probe bundle with the listed
API version and isExposed false. Cap cases declare lightning__dynamicComponent;
nocap cases omit it. Each component, utility, or sibling module uses either a
quoted string literal or a substitution-free template literal for its dynamic
import. Results record deployment outcomes, not browser execution.
