Long-lived personal Developer Edition org (not a fresh scratch org), capture
date 2026-11-10. Route: Lightning Experience custom tab with a synthetic LWC.
The bundle (probe.html, probe.js) was deployed twice, at API 59.0 and 67.0, as
the API in org.tsv. Fields marked data-report had reportValidity() called once
after the first render. combobox_open was opened by clicking its combobox
trigger. The DOM was read through the DevTools protocol with shadow roots
pierced; each row lists which of these classes appear anywhere inside the probe
wrapper: slds-form-element slds-has-error slds-combobox slds-is-open slds-input
slds-textarea slds-is-selected slds-not-selected. The bundles, tabs and the
permission set used to show the tabs were deleted afterwards. No org or user
identifiers are recorded.
