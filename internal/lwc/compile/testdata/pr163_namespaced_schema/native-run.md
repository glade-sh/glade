Capture date: 2026-11-09 UTC.
Route: check-only Salesforce CLI deployment to an authorized existing namespaced
developer org. The org was not a scratch org; no metadata was saved. Each case
validated the synthetic object and LWC together with the listed deployment and
bundle API version. The JS source substitutes the expression column into
probe.js; {namespace} denotes the target org's owning namespace.
The object source is object.xml; bundle metadata set isExposed false.
The non-owning namespace control had no installed package with that name.
Rejected own-namespace imports reported LWC1504. Other rejections were invalid
references. This records deployment compilation, not browser execution.
