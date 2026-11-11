A fresh 30-day Developer scratch org was created for this capture and deleted after use.
Capture date: 2026-11-09 UTC.
Route ui-list-object-info is GET of the Salesforce UI API list-object-info
resource at API 67. For each returned column, input is its alias and outcome
records fieldApiName and displayable. Only these public column properties are
retained. The list reflects this org shape and access, not all possible shapes.
Route deploy is a synthetic ListView metadata deployment using input as its
sole column and the listed deployment API. Rejected rows reported an
unresolved list-view column; accepted rows deployed.
